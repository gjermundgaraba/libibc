package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain/dockerutils"
)

func localnetCmd() *cobra.Command {

	return &cobra.Command{
		Use:   "localnet",
		Short: "Spins up a localnet with a Cosmos chain and an Ethereum chain",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			logWriter.AddExtraLogger(func(entry string) {
				cmd.Println(entry)
			})

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
			if err != nil {
				return fmt.Errorf("failed to start chain: %w", err)
			}

			defer cleanupFunc()

			logger.Info("Chain started", zap.String("chain-id", cfg.ChainID))
			sleepTime := 1 * time.Minute
			logger.Info("Sleeping before killing the chain", zap.Duration("sleep-time", sleepTime))
			time.Sleep(sleepTime)

			return nil
		},
	}
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
