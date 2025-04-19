//go:build e2e

package localnet_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain/dockerutils"
)

func TestLocalnet(t *testing.T) {
	ctx := context.Background()

	logger, err := zap.NewDevelopmentConfig().Build()
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
	cosmosChain := cosmoschain.NewCosmosChain(logger, "test-chain-1", cfg, cleanupLabel, 1, 1)

	cleanupFunc, err := cosmosChain.Start(ctx, logger)
	require.NoError(t, err)
	t.Cleanup(cleanupFunc)

	c, err := cosmos.NewCosmos(logger.With(zap.String("scope", "cosmos-client")), cfg.ChainID, cfg.Bech32Prefix, cfg.Denom, 0, cosmosChain.GetHostGRPCAddress())
	require.NoError(t, err)

	// wallet, err := cosmosChain.BuildWallet(ctx, "test-wallet", "")
	// require.NoError(t, err)

	faucetPrivKeyHex, err := cosmosChain.PrivateKey(ctx, cosmoschain.FaucetKeyName)
	require.NoError(t, err)

	err = c.AddWallet(cosmoschain.FaucetKeyName, faucetPrivKeyHex)
	require.NoError(t, err)

	faucetWallet, err := c.GetWallet(cosmoschain.FaucetKeyName)
	require.NoError(t, err)
	faucetAddress := faucetWallet.Address()

	balance, err := c.GetBalance(ctx, faucetAddress, cfg.Denom)
	require.NoError(t, err)
	require.Equal(t, balance.Uint64(), uint64(100_000_000_000_000))

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
