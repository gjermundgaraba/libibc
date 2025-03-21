package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sync"

	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func scriptCmd() *cobra.Command {
	var (
		numPacketsPerWallet int
		transferAmount      int

		chainAId              string
		chainAClientId        string
		chainADenom           string
		chainARelayerWalletId string

		chainBId              string
		chainBClientId        string
		chainBDenom           string
		chainBRelayerWalletId string
	)

	cmd := &cobra.Command{
		Use:   "script",
		Short: "Run a script",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			tuiInstance := tui.NewTui("Starting script", "Initializing")

			network, err := cfg.ToNetwork(ctx, tuiInstance.GetLogger())
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			transferAmountBig := big.NewInt(int64(transferAmount))
			chainA := network.GetChain(chainAId)
			chainB := network.GetChain(chainBId)

			chainARelayerWallet, err := chainA.GetWallet(chainARelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", chainARelayerWalletId)
			}

			chainBRelayerWallet, err := chainB.GetWallet(chainBRelayerWalletId)
			if err != nil {
				return errors.Wrapf(err, "failed to get wallet %s", chainBRelayerWalletId)
			}

			chainBWallets := chainB.GetWallets()
			chainAWallets := chainA.GetWallets()

			// Limit wallets for testing
			if len(chainBWallets) > 10 {
				chainBWallets = chainBWallets[:10]
			}
			if len(chainAWallets) > 10 {
				chainAWallets = chainAWallets[:10]
			}

			if len(chainBWallets) != len(chainAWallets) {
				return errors.Errorf("wallets length mismatch: %d != %d", len(chainBWallets), len(chainAWallets))
			}

			go func() {
				defer func() {
					if r := recover(); r != nil {
						tuiInstance.GetLogger().Error("Panic", zap.Any("panic", r))
						tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Panic: %v", r))
					}
				}()

				if err := runScript(
					ctx,
					tuiInstance,
					network,
					chainA,
					chainAClientId,
					chainADenom,
					chainAWallets,
					chainARelayerWallet,
					chainB,
					chainBClientId,
					chainBDenom,
					chainBWallets,
					chainBRelayerWallet,
					transferAmountBig,
					numPacketsPerWallet,
				); err != nil {
					tuiInstance.GetLogger().Error("Script failed", zap.Error(err))
					tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Error: %s", err.Error()))
				}
			}()

			if err := tuiInstance.Run(); err != nil {
				fmt.Println("Error running TUI program:", err)
				os.Exit(1)
			}

			// Ensure resources are properly closed
			defer tuiInstance.Close()

			return nil
		},
	}

	cmd.Flags().IntVar(&numPacketsPerWallet, "packets-per-wallet", 5, "Number of packets to send per wallet")
	cmd.Flags().IntVar(&transferAmount, "transfer-amount", 100, "Amount to transfer")
	cmd.Flags().StringVar(&chainAId, "chain-a-id", "11155111", "Chain A ID")
	cmd.Flags().StringVar(&chainAClientId, "chain-a-client-id", "plz-last-hub-devnet-69", "Chain A client ID")
	cmd.Flags().StringVar(&chainADenom, "chain-a-denom", "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14", "Chain A denom")
	cmd.Flags().StringVar(&chainARelayerWalletId, "chain-a-relayer-wallet-id", "eth-relayer", "Chain A relayer wallet ID")
	cmd.Flags().StringVar(&chainBId, "chain-b-id", "eureka-hub-dev-6", "Chain B ID")
	cmd.Flags().StringVar(&chainBClientId, "chain-b-client-id", "08-wasm-2", "Chain B client ID")
	cmd.Flags().StringVar(&chainBDenom, "chain-b-denom", "uatom", "Chain B denom")
	cmd.Flags().StringVar(&chainBRelayerWalletId, "chain-b-relayer-wallet-id", "cosmos-relayer", "Chain B relayer wallet ID")

	return cmd
}

func runScript(
	ctx context.Context,
	tuiInstance *tui.Tui,
	network *network.Network,
	chainA network.Chain,
	chainAClientId string,
	chainADenom string,
	chainAWallets []network.Wallet,
	chainARelayerWallet network.Wallet,
	chainB network.Chain,
	chainBClientId string,
	chainBDenom string,
	chainBWallets []network.Wallet,
	chainBRelayerWallet network.Wallet,
	transferAmountBig *big.Int,
	numPacketsPerWallet int,
) error {
	// Get the logger from the TUI
	tuiLogger := tuiInstance.GetLogger()

	tuiLogger.Info("Starting up", zap.Int("wallet-count", len(chainBWallets)))

	var mainErrGroup errgroup.Group

	tuiInstance.UpdateMainStatus("Transferring...")
	mainErrGroup.Go(func() error {
		return transferAndRelayFromAToB(
			ctx,
			tuiInstance,
			network,
			chainA,
			chainAClientId,
			chainADenom,
			chainAWallets,
			chainB,
			chainBWallets,
			chainBRelayerWallet,
			transferAmountBig,
			numPacketsPerWallet,
		)
	})
	mainErrGroup.Go(func() error {
		return transferAndRelayFromAToB(
			ctx,
			tuiInstance,
			network,
			chainB,
			chainBClientId,
			chainBDenom,
			chainBWallets,
			chainA,
			chainAWallets,
			chainARelayerWallet,
			transferAmountBig,
			numPacketsPerWallet,
		)
	})

	// Wait for everything to complete
	if err := mainErrGroup.Wait(); err != nil {
		tuiLogger.Error("Failed to complete transfers", zap.Error(err))
		tuiInstance.UpdateMainErrorStatus(fmt.Sprintf("Failed to complete transfers: %s", err.Error()))
		return errors.Wrap(err, "failed to complete transfers")
	}

	// Log successful completion
	tuiLogger.Info("All transfers and relays completed successfully")
	tuiInstance.UpdateMainStatus("All transfers and relays completed")

	return nil
}

func transferAndRelayFromAToB(
	ctx context.Context,
	tuiInstance *tui.Tui,
	network *network.Network,
	fromChain network.Chain,
	fromClientId string,
	denom string,
	fromWallets []network.Wallet,
	toChain network.Chain,
	toWallets []network.Wallet,
	toChainRelayerWallet network.Wallet,
	transferAmount *big.Int,
	numPacketsPerWallet int,
) error {
	tuiLogger := tuiInstance.GetLogger()
	relayerQueue := network.NewRelayerQueue(tuiLogger, fromChain, toChain, toChainRelayerWallet, 5)

	aToBUpdateMutext := sync.Mutex{}

	totalTransfer := len(toWallets) * numPacketsPerWallet
	transferCompleted := 0
	transferStatusModel := tui.NewStatusModel(fmt.Sprintf("Transferring from %s to %s 0/%d", fromChain.GetChainID(), toChain.GetChainID(), totalTransfer))
	tuiInstance.AddStatusModel(transferStatusModel)

	relayingStatusModel := tui.NewStatusModel(fmt.Sprintf("Relaying from %s to chain %s 0/%d", fromChain.GetChainID(), toChain.GetChainID(), totalTransfer))
	tuiInstance.AddStatusModel(relayingStatusModel)

	errGroup := errgroup.Group{}

	for i := 0; i < len(toWallets); i++ {
		idx := i
		errGroup.Go(func() error {
			chainAWallet := fromWallets[idx]
			chainBWallet := toWallets[idx]

			for i := 0; i < numPacketsPerWallet; i++ {
				var packet ibc.Packet
				if err := withRetry(func() error {
					var err error
					packet, err = fromChain.SendTransfer(ctx, fromClientId, chainAWallet, transferAmount, denom, chainBWallet.Address())
					return err
				}); err != nil {
					return errors.Wrapf(err, "failed to create transfer from %s to chain %s", fromChain.GetChainID(), toChain.GetChainID())
				}
				relayerQueue.Add(packet)

				aToBUpdateMutext.Lock()
				transferCompleted++
				transferStatusModel.UpdateStatus(fmt.Sprintf("Transferring from %s to %s (%d/%d)", fromChain.GetChainID(), toChain.GetChainID(), transferCompleted, totalTransfer))
				transferStatusModel.UpdateProgress(int(transferCompleted * 100 / totalTransfer))

				inQueue, currentlyRelaying, completedRelaying := relayerQueue.Status()
				relayingStatusModel.UpdateStatus(fmt.Sprintf("Relaying from %s to %s %d/%d (waiting: %d)", fromChain.GetChainID(), toChain.GetChainID(), completedRelaying+currentlyRelaying, totalTransfer, inQueue))
				relayingStatusModel.UpdateProgress(int(completedRelaying * 100 / totalTransfer))
				aToBUpdateMutext.Unlock()

				tuiLogger.Info("Transferred completed",
					zap.String("from-chain", fromChain.GetChainID()),
					zap.String("from-client", fromClientId),
					zap.String("to-chain", toChain.GetChainID()),
					zap.Int("current-a-to-b-transfer", transferCompleted),
					zap.Int("total-a-to-b-transfer", totalTransfer),
					zap.String("from", chainAWallet.Address()),
					zap.String("from-id", chainAWallet.ID()),
					zap.String("to-id", chainBWallet.ID()),
					zap.String("to", chainBWallet.Address()),
					zap.String("amount", transferAmount.String()),
				)
			}

			return nil
		})
	}

	tuiLogger.Info(fmt.Sprintf("Waiting for transfers to complete from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))
	transferStatusModel.UpdateStatus(fmt.Sprintf("Waiting for transfers to complete from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))
	if err := errGroup.Wait(); err != nil {
		tuiLogger.Error("Failed to complete transfers", zap.Error(err))
		transferStatusModel.UpdateErrorStatus(fmt.Sprintf("Failed: %s", err.Error()))
		return errors.Wrap(err, "failed to complete transfers")
	}
	tuiLogger.Info(fmt.Sprintf("Transfers completed from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))
	transferStatusModel.UpdateStatus(fmt.Sprintf("Transfers completed from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))

	inQueue, currentlyRelaying, completedRelaying := relayerQueue.Status()
	tuiLogger.Info("Flushing queue", zap.String("from-chain", fromChain.GetChainID()), zap.String("to-chain", toChain.GetChainID()), zap.Int("in-queue", inQueue), zap.Int("currently-relaying", currentlyRelaying), zap.Int("completed-relaying", completedRelaying))
	relayingStatusModel.UpdateStatus(fmt.Sprintf("Flushing relay queue from %s to %s %d/%d (waiting: %d)", fromChain.GetChainID(), toChain.GetChainID(), completedRelaying+currentlyRelaying, totalTransfer, inQueue))
	if err := relayerQueue.Flush(); err != nil {
		tuiLogger.Error("Failed to flush queue", zap.Error(err))
		relayingStatusModel.UpdateErrorStatus(fmt.Sprintf("Queue flush failed: %s", err.Error()))
		return errors.Wrap(err, "failed to flush queue")
	}

	// Log and update status for queue flush completion
	tuiLogger.Info("Queue flushed successfully",
		zap.String("from-chain", fromChain.GetChainID()),
		zap.String("to-chain", toChain.GetChainID()),
		zap.Int("completed-packets", totalTransfer))

	// Update status with 100% completion
	relayingStatusModel.UpdateStatus(fmt.Sprintf("Relay queue flushed from %s to %s %d/%d", fromChain.GetChainID(), toChain.GetChainID(), totalTransfer, totalTransfer))
	relayingStatusModel.UpdateProgress(100)

	return nil
}

func withRetry(f func() error) error {
	const maxRetries = 3
	var err error
	for range maxRetries {
		err = f()
		if err == nil {
			return nil
		}
	}

	return err
}
