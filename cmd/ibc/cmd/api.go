package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	skipapi "github.com/gjermundgaraba/libibc/apis/skip-api"
)

func apiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api [from-chain-id] [to-chain-id] [source-client] [from-wallet-id] [amount] [src-denom] [dest-denom] [to-address]",
		Short: "simple command to interact with the skip api",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			logWriter.AddExtraLogger(func(entry string) {
				fmt.Println(entry)
			})

			network, err := cfg.ToNetwork(ctx, logger, extraGwei)
			if err != nil {
				return err
			}

			fromChainID := args[0]
			fromChain, err := network.GetChain(fromChainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", fromChainID)
			}

			toChainID := args[1]
			_ = args[2]
			fromWalletID := args[3]
			fromWallet, err := fromChain.GetWallet(fromWalletID)
			if err != nil {
				return errors.Wrapf(err, "failed to get from-wallet %s", fromWalletID)
			}
			amountStr := args[4]
			srcDenom := args[5]
			destDenom := args[6]
			toAddress := args[7]

			skipClient := skipapi.NewClient(logger, "https://api.skip.money")

			routeResp, err := skipClient.Route(ctx, skipapi.RouteRequest{
				SourceAssetDenom:   srcDenom,
				SourceAssetChainID: fromChainID,
				DestAssetDenom:     destDenom,
				DestAssetChainID:   toChainID,
				AllowUnsafe:        true,
				ExperimentalFeatures: []string{
					"stargate", "eureka", "hyperlane",
				},
				AllowMultiTx: true,
				SmartRelay:   true,
				SmartSwapOptions: skipapi.SmartSwapOptions{
					SplitRoutes: true,
					EvmSwaps:    true,
				},
				GoFast:   true,
				AmountIn: amountStr,
			})
			if err != nil {
				return errors.Wrapf(err, "failed to get route")
			}

			msgsResp, err := skipClient.Msgs(ctx, skipapi.MsgsRequest{
				SourceAssetDenom:   srcDenom,
				SourceAssetChainID: fromChainID,
				DestAssetDenom:     destDenom,
				DestAssetChainID:   toChainID,
				AmountIn:           amountStr,
				AmountOut:          routeResp.EstimatedAmountOut,
				AddressList: []string{
					fromWallet.Address(),
					toAddress,
				},
				Operations: routeResp.Operations,
			})
			if err != nil {
				return err
			}

			fmt.Printf("msgsResp: %v\n", msgsResp)

			return nil
		},
	}
	return cmd
}
