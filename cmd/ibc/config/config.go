package config

import (
	"context"
	"fmt"
	"os"

	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/gjermundgaraba/libibc/chains/ethereum"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/relayer"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	Chains          []ChainConfig  `toml:"chains"`
	Wallets         []WalletConfig `toml:"wallets"`
	RelayerGRPCAddr string         `toml:"relayer-grpc-addr"`
}

// ChainConfig represents the configuration for a single chain
type ChainConfig struct {
	ChainType string         `toml:"chain-type"`
	ChainID   string         `toml:"chain-id"`
	RPCAddr   string         `toml:"rpc-addr"`
	GRPCAddr  string         `toml:"grpc-addr"`
	Clients   []ClientConfig `toml:"clients"`
	WalletIDs []string       `toml:"wallet-ids"`

	// TODO: Find a way to put chain specific fields somwhere else?

	// Cosmos specific fields
	Bech32Prefix string `toml:"bech32-prefix"`
	GasDenom     string `toml:"gas-denom"`

	// Ethereum specific fields
	ICS26Address         string `toml:"ics26-address"`
	RelayerHelperAddress string `toml:"relayer-helper-address"`
}

// ClientConfig represents the configuration for a client
type ClientConfig struct {
	ClientID             string `toml:"client-id"`
	CounterpartyChainID  string `toml:"counterparty-chain-id"`
	CounterpartyClientID string `toml:"counterparty-client-id"`
}

// WalletConfig represents the configuration for a wallet
type WalletConfig struct {
	WalletID   string `toml:"wallet-id"`
	PrivateKey string `toml:"private-key"`
}

// LoadConfig reads and parses the config file
func LoadConfig(configPath string) (*Config, error) {
	// Read the config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	if err := toml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &config, nil
}

// SaveConfig writes the config to file using go-toml directly
func (c *Config) SaveConfig(configPath string) error {
	// Marshal directly to TOML
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to a new temporary file for atomic write
	tempFile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write the TOML data to the temp file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Close the temp file
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Copy the temp file to the destination
	if err := os.Rename(tempFile.Name(), configPath); err != nil {
		// If rename fails (e.g., across filesystems), try copy
		input, err := os.ReadFile(tempFile.Name())
		if err != nil {
			return fmt.Errorf("failed to read temp file: %w", err)
		}

		if err := os.WriteFile(configPath, input, 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}

	return nil
}

func (c *Config) ToNetwork(ctx context.Context, logger *zap.Logger, extraGwei int64) (*network.Network, error) {
	walletConfigs := make(map[string]WalletConfig)
	for _, walletConfig := range c.Wallets {
		walletConfigs[walletConfig.WalletID] = walletConfig
	}

	var chains []network.Chain
	for _, chainConfig := range c.Chains {
		var (
			chain network.Chain
			err   error
		)
		switch chainConfig.ChainType {
		case "cosmos":
			chain, err = cosmos.NewCosmos(logger, chainConfig.ChainID, chainConfig.Bech32Prefix, chainConfig.GasDenom, chainConfig.GRPCAddr)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create Cosmos chain")
			}
		case "ethereum":
			ethChain, err := ethereum.NewEthereum(ctx, logger, chainConfig.ChainID, chainConfig.RPCAddr, chainConfig.ICS26Address, chainConfig.RelayerHelperAddress)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create Ethereum chain")
			}
			ethChain.SetExtraGwei(extraGwei)
			chain = ethChain
		default:
			panic(fmt.Sprintf("unsupported chain type: %s", chainConfig.ChainType))
		}

		for _, clientConfig := range chainConfig.Clients {
			counterparty := network.ClientCounterparty{
				ClientID: clientConfig.CounterpartyClientID,
				ChainID:  clientConfig.CounterpartyChainID,
			}
			chain.AddClient(clientConfig.ClientID, counterparty)
		}

		for _, walletID := range chainConfig.WalletIDs {
			walletConfig, ok := walletConfigs[walletID]
			if !ok {
				return nil, fmt.Errorf("wallet config not found for wallet ID: %s for chain %s", walletID, chainConfig.ChainID)
			}
			if err := chain.AddWallet(walletID, walletConfig.PrivateKey); err != nil {
				return nil, errors.Wrap(err, "failed to add wallet to chain")
			}
		}

		chains = append(chains, chain)

	}

	relayer := relayer.NewRelayer(logger, c.RelayerGRPCAddr)
	return network.BuildNetwork(logger, chains, relayer)
}
