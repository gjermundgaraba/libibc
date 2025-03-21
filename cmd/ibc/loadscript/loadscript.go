package loadscript

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"cosmossdk.io/errors"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/cmd/ibc/tui"
	"github.com/gjermundgaraba/libibc/ibc"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func RunScript(
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
