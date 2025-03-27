package cmd

import (
	"fmt"
	"math/big"
	"os"

	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func transferCmd() *cobra.Command {
	var (
		selfRelay     bool
		relayWalletID string
	)

	cmd := &cobra.Command{
		Use:   "transfer [from-chain-id] [to-chain-id] [source-client] [from-wallet-id] [amount] [denom] [to-address] [memo]",
		Args:  cobra.ExactArgs(8),
		Short: "Transfer tokens between two chains",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			tuiInstance := tui.NewTui(logWriter, "Starting script", "Initializing")

			fromChainID := args[0]
			toChainID := args[1]
			sourceClient := args[2]
			fromWalletID := args[3]
			amountStr := args[4]
			denom := args[5]
			toAddress := args[6]
			memo := args[7]

			networkConfig, err := cfg.ToNetwork(ctx, logger)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			fromChain, err := networkConfig.GetChain(fromChainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", fromChainID)
			}

			toChain, err := networkConfig.GetChain(toChainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", toChainID)
			}

			fromWallet, err := fromChain.GetWallet(fromWalletID)
			if err != nil {
				return errors.Wrapf(err, "failed to get from-wallet %s", fromWalletID)
			}

			amount, ok := new(big.Int).SetString(amountStr, 10)
			if !ok {
				return errors.Errorf("failed to parse amount %s", amountStr)
			}

			var relayerWallet network.Wallet
			if selfRelay {
				relayerWallet, err = toChain.GetWallet(relayWalletID)
				if err != nil {
					return errors.Wrapf(err, "failed to get relayer wallet %s", relayWalletID)
				}
			}

			relayer := networkConfig.NewRelayerQueue(logger, fromChain, toChain, relayerWallet, 1, selfRelay)

			go func() {
				errGroup := errgroup.Group{}

				errGroup.Go(func() error {
					logger.Info("Sending transfer", zap.String("from-chain", fromChainID), zap.String("to-chain", toChainID), zap.String("from-wallet", fromWalletID), zap.String("amount", amount.String()), zap.String("denom", denom), zap.String("to-address", toAddress))

					tuiInstance.UpdateMainStatus("Transferring...")

					packet, err := fromChain.SendTransfer(ctx, sourceClient, fromWallet, amount, denom, toAddress, memo)
					if err != nil {
						return errors.Wrap(err, "failed to send transfer")
					}

					logger.Info("Sent transfer", zap.String("tx-hash", packet.TxHash), zap.Uint64("sequence", packet.Sequence), zap.String("dest-client", packet.DestinationClient))

					tuiInstance.UpdateProgress(50)

					if selfRelay {
						tuiInstance.UpdateMainStatus("Self-relaying...")
					} else {
						tuiInstance.UpdateMainStatus("Waiting for relay...")
					}

					relayer.Add(packet)

					if err := relayer.Flush(); err != nil {
						return errors.Wrap(err, "failed to flush relayer")
					}

					logger.Info("Transfer completed")
					tuiInstance.UpdateMainStatus("Transfer completed with relay")
					tuiInstance.UpdateProgress(100)

					return nil
				})

				if err := errGroup.Wait(); err != nil {
					logger.Error("Failed to complete transfers", zap.Error(err))
					tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Failed to complete transfers: %s", err.Error()))
				}

			}()

			if err := tuiInstance.Run(); err != nil {
				fmt.Println("Error running TUI program:", err)
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&selfRelay, "self-relay", false, "Relay by calling the relayer directly, if not set will wait for packet to get picked up")
	cmd.Flags().StringVar(&relayWalletID, "relayer-wallet", "", "Wallet ID to use for relaying")

	return cmd
}
