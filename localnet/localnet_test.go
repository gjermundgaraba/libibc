//go:build e2e

package localnet_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/gjermundgaraba/libibc/chainclients/cosmos"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain/dockerutils"
	"github.com/gjermundgaraba/libibc/localnet/ethereumchain"
	"github.com/gjermundgaraba/libibc/localnet/eurekarelayer"
)

func TestCosmosOnlyLocalnet(t *testing.T) {
	ctx := context.Background()

	logger, err := testLogger()
	require.NoError(t, err)

	cleanupLabel := "cli-localnet"
	cfg := cosmoschain.ChainConfig{
		Name:    "ibc-go-simd-1",
		ChainID: "simd-1",
		Image: dockerutils.ImageRef{
			Repository: "ghcr.io/cosmos/ibc-go-wasm-simd",
			Tag:        "release-v10.1.x",
			UidGid:     "1025:1025",
		},
		Bin:            "simd",
		Bech32Prefix:   "cosmos",
		Denom:          "stake",
		GasPrices:      "0.00stake",
		GasAdjustment:  1.3,
		EncodingConfig: cosmos.SDKEncodingConfig(),
		ModifyGenesis:  defaultModifyGenesis(),
		TrustingPeriod: "508h",
		CoinDecimals:   6,
		CoinType:       "118",
	}
	cosmosChain, cleanupFunc, err := cosmoschain.SpinUpCosmos(ctx, logger, "test-chain-1", cfg, cleanupLabel, 1, 1)
	require.NoError(t, err)
	t.Cleanup(cleanupFunc)

	cosmosClient := cosmosChain.ChainClient

	testWallet, err := cosmosClient.GenerateWallet("test-wallet")
	require.NoError(t, err)
	testAddress := testWallet.Address()
	logger.Info("Test wallet address", zap.String("address", testAddress))

	faucetWallet, err := cosmosClient.GetWallet(cosmoschain.FaucetKeyName)
	require.NoError(t, err)
	faucetAddress := faucetWallet.Address()

	faucetBalance, err := cosmosClient.GetBalance(ctx, faucetAddress, cfg.Denom)
	require.NoError(t, err)
	require.Equal(t, faucetBalance.Uint64(), uint64(100_000_000_000_000))

	testBalance, err := cosmosClient.GetBalance(ctx, testAddress, cfg.Denom)
	require.NoError(t, err)
	require.Equal(t, testBalance.Uint64(), uint64(0))
}

func TestCosmosAndEthereumLocalnet(t *testing.T) {
	ctx := context.Background()

	logger, err := testLogger()
	require.NoError(t, err)

	// Start Cosmos chain
	cleanupLabel := "cli-localnet"
	cosmosCfg := cosmoschain.ChainConfig{
		Name:    "ibc-go-simd-1",
		ChainID: "simd-1",
		Image: dockerutils.ImageRef{
			Repository: "ghcr.io/cosmos/ibc-go-wasm-simd",
			Tag:        "release-v10.1.x",
			UidGid:     "1025:1025",
		},
		Bin:            "simd",
		Bech32Prefix:   "cosmos",
		Denom:          "stake",
		GasPrices:      "0.00stake",
		GasAdjustment:  1.3,
		EncodingConfig: cosmos.SDKEncodingConfig(),
		ModifyGenesis:  defaultModifyGenesis(),
		TrustingPeriod: "508h",
		CoinDecimals:   6,
		CoinType:       "118",
	}
	cosmosChain, cleanupFunc, err := cosmoschain.SpinUpCosmos(ctx, logger, "test-chain-1", cosmosCfg, cleanupLabel, 1, 1)
	require.NoError(t, err)
	t.Cleanup(cleanupFunc)

	cosmosClient := cosmosChain.ChainClient

	// Start Ethereum chain
	ethereumCfg := ethereumchain.NetworkParams{
		Participants: []ethereumchain.Participant{
			{
				CLType:         "lodestar",
				CLImage:        "ethpandaops/lodestar:unstable",
				ELType:         "geth",
				ELImage:        "ethpandaops/geth:prague-devnet-6",
				ELExtraParams:  []string{"--gcmode=archive"},
				ELLogLevel:     "info",
				ValidatorCount: 64,
			},
		},
		NetworkParams: ethereumchain.NetworkConfigParams{
			Preset:           "minimal",
			ElectraForkEpoch: 1,
		},
		WaitForFinalization: true,
		AdditionalServices:  []string{},
	}
	ethereumChain, err := ethereumchain.SpinUpEthereum(ctx, logger, ethereumCfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		ethereumChain.Destroy(ctx)
	})

	ethereumClient := ethereumChain.ChainClient

	cosmosFaucetWallet, err := cosmosClient.GetWallet(cosmoschain.FaucetKeyName)
	require.NoError(t, err)
	ethereumFaucetWallet, err := ethereumClient.GetWallet(ethereumchain.FaucetKeyName)
	require.NoError(t, err)

	cosmosRelayerWallet, err := cosmosClient.GenerateWallet("relayer")
	require.NoError(t, err)
	_, err = cosmosClient.Send(ctx, cosmosFaucetWallet, big.NewInt(100_000_000_000), cosmosCfg.Denom, cosmosRelayerWallet.Address())
	require.NoError(t, err)
	cosmosRelayerBalance, err := cosmosClient.GetBalance(ctx, cosmosRelayerWallet.Address(), cosmosCfg.Denom)
	require.NoError(t, err)
	require.Equal(t, uint64(100_000_000_000), cosmosRelayerBalance.Uint64())

	ethereumRelayerWallet, err := ethereumClient.GenerateWallet("relayer")
	require.NoError(t, err)
	_, err = ethereumClient.Send(ctx, ethereumFaucetWallet, big.NewInt(100_000_000_000), "eth", ethereumRelayerWallet.Address())
	require.NoError(t, err)
	ethereumRelayerBalance, err := ethereumClient.GetBalance(ctx, ethereumRelayerWallet.Address(), "eth")
	require.NoError(t, err)
	require.Equal(t, uint64(100_000_000_000), ethereumRelayerBalance.Uint64())

	sp1Config := eurekarelayer.SP1ProverConfig{
		Type:           eurekarelayer.SP1ProverTypeNetwork,
		PrivateCluster: true,
	}

	config := eurekarelayer.NewConfig(
		"debug",
		3000,
		eurekarelayer.CreateEthCosmosModules(
			eurekarelayer.EthCosmosConfigInfo{
				EthChainID:     ethereumClient.ChainID,
				CosmosChainID:  cosmosClient.ChainID,
				TmRPC:          cosmosChain.GetHostRPCAddress(),
				ICS26Address:   ethereumClient.ICS26Address.Hex(),
				EthRPC:         ethereumClient.RPC,
				BeaconAPI:      ethereumChain.BeaconRPC,
				SP1Config:      sp1Config,
				SignerAddress:  cosmosRelayerWallet.Address(),
				MockWasmClient: false,
			}),
	)

	tmpDir := t.TempDir()
	configFilePath := fmt.Sprintf("%s/config.json", tmpDir)
	err = config.GenerateConfigFile(configFilePath)
	require.NoError(t, err)

	relayerProcess, err := eurekarelayer.StartRelayer(configFilePath)
	require.NoError(t, err)

	t.Cleanup(func() {
		if relayerProcess != nil {
			err := relayerProcess.Kill()
			if err != nil {
				logger.Error("failed to kill relayer process", zap.Error(err))
			} else {
				logger.Debug("relayer process killed")
			}
		}
	})

}

func testLogger() (*zap.Logger, error) {
	logConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	return logConfig.Build()
}

func defaultModifyGenesis() func(cosmoschain.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig cosmoschain.ChainConfig, genBz []byte) ([]byte, error) {
		appGenesis, err := genutiltypes.AppGenesisFromReader(bytes.NewReader(genBz))
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis bytes: %w", err)
		}

		var appState genutiltypes.AppMap
		if err := json.Unmarshal(appGenesis.AppState, &appState); err != nil {
			return nil, fmt.Errorf("failed to unmarshal app state: %w", err)
		}

		// modify the gov v1 app state
		govGenBz, err := modifyGovV1AppState(chainConfig, appState[govtypes.ModuleName])
		if err != nil {
			return nil, fmt.Errorf("failed to modify gov v1 app state: %w", err)
		}

		appState[govtypes.ModuleName] = govGenBz

		// marshal the app state
		appGenesis.AppState, err = json.Marshal(appState)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal app state: %w", err)
		}

		res, err := json.MarshalIndent(appGenesis, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal app genesis: %w", err)
		}

		return res, nil
	}
}

// modifyGovV1AppState takes the existing gov app state and marshals it to a govv1 GenesisState.
func modifyGovV1AppState(chainConfig cosmoschain.ChainConfig, govAppState []byte) ([]byte, error) {
	cdc := cosmos.SDKEncodingConfig().Codec

	govGenesisState := &govv1.GenesisState{}
	if err := cdc.UnmarshalJSON(govAppState, govGenesisState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal genesis bytes into gov genesis state: %w", err)
	}

	if govGenesisState.Params == nil {
		govGenesisState.Params = &govv1.Params{}
	}

	govGenesisState.Params.MinDeposit = sdk.NewCoins(sdk.NewCoin(chainConfig.Denom, govv1.DefaultMinDepositTokens))
	maxDepositPeriod := time.Second * 10
	votingPeriod := time.Second * 30
	govGenesisState.Params.MaxDepositPeriod = &maxDepositPeriod
	govGenesisState.Params.VotingPeriod = &votingPeriod

	// govGenBz := MustProtoMarshalJSON(govGenesisState)

	govGenBz, err := cdc.MarshalJSON(govGenesisState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gov genesis state: %w", err)
	}

	return govGenBz, nil
}
