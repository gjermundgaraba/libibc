package loadscript

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	skipapi "github.com/gjermundgaraba/libibc/apis/skip-api"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/gjermundgaraba/libibc/relayer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Stage uint8

type ProgressUpdate struct {
	UpdateType Stage

	FromChain         string
	ToChain           string
	CurrentTransfers  int
	TotalTransfers    int
	CompletedRelaying int
	InQueueRelays     int
	ErrorMessage      string
}

const (
	ErrorUpdate Stage = iota
	TransferUpdate
	RelayingUpdate
	DoneUpdate
)

func TransferAndRelayFromAToB(
	ctx context.Context,
	logger *zap.Logger,
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
	selfRelay bool,
	eurekaRelayerAddr string,
	skipApiAddr string,
) (chan ProgressUpdate, error) {
	relayerQueue := relayer.NewRelayerQueue(logger, fromChain, toChain, toChainRelayerWallet, 10, selfRelay, eurekaRelayerAddr)
	progressCh := make(chan ProgressUpdate, 100)

	aToBUpdateMutext := sync.Mutex{}

	totalTransfer := len(toWallets) * numPacketsPerWallet
	transferCompleted := 0

	progressCh <- ProgressUpdate{
		FromChain:         fromChain.GetChainID(),
		ToChain:           toChain.GetChainID(),
		CurrentTransfers:  0,
		TotalTransfers:    totalTransfer,
		CompletedRelaying: 0,
		InQueueRelays:     0,
		UpdateType:        TransferUpdate,
	}

	reportErr := func(err error) {
		progressCh <- ProgressUpdate{
			UpdateType:     ErrorUpdate,
			FromChain:      fromChain.GetChainID(),
			ToChain:        toChain.GetChainID(),
			TotalTransfers: totalTransfer,
			ErrorMessage:   err.Error(),
		}
	}

	go func() {
		errGroup := errgroup.Group{}

		for i := range toWallets {
			idx := i
			errGroup.Go(func() error {
				chainAWallet := fromWallets[idx]
				chainBWallet := toWallets[idx]

				for range numPacketsPerWallet {
					var packet ibc.Packet
					if err := withRetry(func() error {
						var err error
						packet, err = transfer(ctx, logger, fromChain, fromClientId, chainAWallet, transferAmount, denom, chainBWallet.Address(), skipApiAddr)
						return err
					}); err != nil {
						reportErr(err)
						return errors.Wrapf(err, "failed to create transfer from %s to chain %s", fromChain.GetChainID(), toChain.GetChainID())
					}
					relayerQueue.Add(packet)

					aToBUpdateMutext.Lock()
					transferCompleted++

					inQueue, _, completedRelaying := relayerQueue.Status()

					progressCh <- ProgressUpdate{
						UpdateType:        TransferUpdate,
						FromChain:         fromChain.GetChainID(),
						ToChain:           toChain.GetChainID(),
						CurrentTransfers:  transferCompleted,
						TotalTransfers:    totalTransfer,
						CompletedRelaying: completedRelaying,
						InQueueRelays:     inQueue,
					}
					aToBUpdateMutext.Unlock()

					logger.Info("Transferred completed",
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

		progressCh <- ProgressUpdate{
			UpdateType:       TransferUpdate,
			FromChain:        fromChain.GetChainID(),
			ToChain:          toChain.GetChainID(),
			CurrentTransfers: transferCompleted,
			TotalTransfers:   totalTransfer,
		}

		logger.Info(fmt.Sprintf("Waiting for transfers to complete from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))
		if err := errGroup.Wait(); err != nil {
			logger.Error("Failed to complete transfers", zap.Error(err))
			reportErr(err)

			close(progressCh)
			return
		}

		logger.Info(fmt.Sprintf("Transfers completed from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))
		progressCh <- ProgressUpdate{
			FromChain:        fromChain.GetChainID(),
			ToChain:          toChain.GetChainID(),
			CurrentTransfers: totalTransfer,
			TotalTransfers:   totalTransfer,
			UpdateType:       RelayingUpdate,
		}

		inQueue, currentlyRelaying, completedRelaying := relayerQueue.Status()
		logger.Info("Flushing queue",
			zap.String("from-chain", fromChain.GetChainID()),
			zap.String("to-chain", toChain.GetChainID()),
			zap.Int("in-queue", inQueue),
			zap.Int("currently-relaying", currentlyRelaying),
			zap.Int("completed-relaying", completedRelaying))

		progressCh <- ProgressUpdate{
			UpdateType:        RelayingUpdate,
			FromChain:         fromChain.GetChainID(),
			ToChain:           toChain.GetChainID(),
			CurrentTransfers:  totalTransfer,
			TotalTransfers:    totalTransfer,
			CompletedRelaying: completedRelaying,
			InQueueRelays:     inQueue,
		}

		if err := relayerQueue.Flush(); err != nil {
			logger.Error("Failed to flush queue", zap.Error(err))
			reportErr(err)

			close(progressCh)
			return
		}

		logger.Info("Queue flushed successfully",
			zap.String("from-chain", fromChain.GetChainID()),
			zap.String("to-chain", toChain.GetChainID()),
			zap.Int("completed-packets", totalTransfer))

		progressCh <- ProgressUpdate{
			UpdateType:        DoneUpdate,
			FromChain:         fromChain.GetChainID(),
			ToChain:           toChain.GetChainID(),
			CurrentTransfers:  totalTransfer,
			TotalTransfers:    totalTransfer,
			CompletedRelaying: totalTransfer,
			InQueueRelays:     0,
		}

		close(progressCh)
	}()

	return progressCh, nil
}

func transfer(ctx context.Context, logger *zap.Logger, chain network.Chain, srcClientID string, wallet network.Wallet, amount *big.Int, denom string, to string, skipAPIAddr string) (ibc.Packet, error) {
	if skipAPIAddr != "" {
		skipAPIClient := skipapi.NewClient(logger, skipAPIAddr)
		txBz, err := skipAPIClient.GetTransferTxs(ctx, denom, chain.GetChainID(), denom, chain.GetChainID(), wallet.Address(), to, amount)
		if err != nil {
			return ibc.Packet{}, errors.Wrapf(err, "failed to get transfer txs from %s to %s", chain.GetChainID(), chain.GetChainID())
		}

		txHash, err := chain.SubmitTx(ctx, txBz, wallet)
		if err != nil {
			return ibc.Packet{}, errors.Wrapf(err, "failed to submit transfer tx from %s to %s", chain.GetChainID(), chain.GetChainID())
		}

		packets, err := chain.GetPackets(ctx, txHash)
		if err != nil {
			return ibc.Packet{}, errors.Wrapf(err, "failed to get packets from %s to %s", chain.GetChainID(), chain.GetChainID())
		}

		if len(packets) == 0 {
			return ibc.Packet{}, errors.New("no packets found")
		}
		if len(packets) > 1 {
			return ibc.Packet{}, errors.New("multiple packets found")
		}

		return packets[0], nil
	} else {
		return chain.SendTransfer(ctx, srcClientID, wallet, amount, denom, to, "")
	}

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
