package config

import (
	"context"
	"fmt"

	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/gjermundgaraba/libibc/chains/ethereum"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/relayer"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Chains          []ChainConfig  `mapstructure:"chains"`
	Wallets         []WalletConfig `mapstructure:"wallets"`
	RelayerGRPCAddr string         `mapstructure:"relayer-grpc-addr"`
}

// ChainConfig represents the configuration for a single chain
type ChainConfig struct {
	ChainType string         `mapstructure:"chain-type"`
	ChainID   string         `mapstructure:"chain-id"`
	RPCAddr   string         `mapstructure:"rpc-addr"`
	GRPCAddr  string         `mapstructure:"grpc-addr"`
	Clients   []ClientConfig `mapstructure:"clients"`
	WalletIDs []string       `mapstructure:"wallet-ids"`

	// Ethereum specific fields
	ICS26Address string `mapstructure:"ics26-address"`
}

// ClientConfig represents the configuration for a client
type ClientConfig struct {
	ClientID             string `mapstructure:"client-id"`
	CounterpartyChainID  string `mapstructure:"counterparty-chain-id"`
	CounterpartyClientID string `mapstructure:"counterparty-client-id"`
}

// WalletConfig represents the configuration for a wallet
type WalletConfig struct {
	WalletID   string `mapstructure:"wallet-id"`
	PrivateKey string `mapstructure:"private-key"`
}

// LoadConfig reads and parses the config file
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func (c *Config) ToNetwork(ctx context.Context) (*network.Network, error) {
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
			chain, err = cosmos.NewCosmos(chainConfig.ChainID, chainConfig.GRPCAddr)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create Cosmos chain")
			}
		case "ethereum":
			chain, err = ethereum.NewEthereum(ctx, chainConfig.ChainID, chainConfig.RPCAddr, chainConfig.ICS26Address)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create Ethereum chain")
			}
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

	relayer := relayer.NewRelayer(c.RelayerGRPCAddr)
	return network.BuildNetwork(chains, relayer)
}
