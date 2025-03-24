package loadscript

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"cosmossdk.io/errors"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/ibc"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type ProgressUpdate struct {
	FromChain        string
	ToChain          string
	CurrentTransfers int
	TotalTransfers   int
	CurrentRelays    int
	InQueueRelays    int
	IsCompleted      bool
	IsError          bool
	ErrorMessage     string
	Stage            string
}

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
) (chan ProgressUpdate, error) {
	relayerQueue := network.NewRelayerQueue(logger, fromChain, toChain, toChainRelayerWallet, 10, selfRelay)
	progressCh := make(chan ProgressUpdate, 100)

	aToBUpdateMutext := sync.Mutex{}

	totalTransfer := len(toWallets) * numPacketsPerWallet
	transferCompleted := 0

	progressCh <- ProgressUpdate{
		FromChain:        fromChain.GetChainID(),
		ToChain:          toChain.GetChainID(),
		CurrentTransfers: 0,
		TotalTransfers:   totalTransfer,
		CurrentRelays:    0,
		InQueueRelays:    0,
		Stage:            "transfer",
	}

	go func() {
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
						progressCh <- ProgressUpdate{
							FromChain:      fromChain.GetChainID(),
							ToChain:        toChain.GetChainID(),
							TotalTransfers: totalTransfer,
							IsError:        true,
							ErrorMessage:   err.Error(),
							Stage:          "error",
						}
						return errors.Wrapf(err, "failed to create transfer from %s to chain %s", fromChain.GetChainID(), toChain.GetChainID())
					}
					relayerQueue.Add(packet)

					aToBUpdateMutext.Lock()
					transferCompleted++
					
					inQueue, _, completedRelaying := relayerQueue.Status()
					
					progressCh <- ProgressUpdate{
						FromChain:        fromChain.GetChainID(),
						ToChain:          toChain.GetChainID(),
						CurrentTransfers: transferCompleted,
						TotalTransfers:   totalTransfer,
						CurrentRelays:    completedRelaying,
						InQueueRelays:    inQueue,
						Stage:            "transfer",
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
			FromChain:        fromChain.GetChainID(),
			ToChain:          toChain.GetChainID(),
			CurrentTransfers: transferCompleted,
			TotalTransfers:   totalTransfer,
			Stage:            "transfer",
		}
		
		logger.Info(fmt.Sprintf("Waiting for transfers to complete from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))
		if err := errGroup.Wait(); err != nil {
			logger.Error("Failed to complete transfers", zap.Error(err))
			
			progressCh <- ProgressUpdate{
				FromChain:      fromChain.GetChainID(),
				ToChain:        toChain.GetChainID(),
				TotalTransfers: totalTransfer,
				IsError:        true,
				ErrorMessage:   err.Error(),
				Stage:          "error",
			}
			
			close(progressCh)
			return
		}
		
		logger.Info(fmt.Sprintf("Transfers completed from %s to %s", fromChain.GetChainID(), toChain.GetChainID()))
		progressCh <- ProgressUpdate{
			FromChain:        fromChain.GetChainID(),
			ToChain:          toChain.GetChainID(),
			CurrentTransfers: totalTransfer,
			TotalTransfers:   totalTransfer,
			Stage:            "relaying",
		}

		inQueue, currentlyRelaying, completedRelaying := relayerQueue.Status()
		logger.Info("Flushing queue", 
			zap.String("from-chain", fromChain.GetChainID()), 
			zap.String("to-chain", toChain.GetChainID()), 
			zap.Int("in-queue", inQueue), 
			zap.Int("currently-relaying", currentlyRelaying), 
			zap.Int("completed-relaying", completedRelaying))
		
		progressCh <- ProgressUpdate{
			FromChain:        fromChain.GetChainID(),
			ToChain:          toChain.GetChainID(),
			CurrentTransfers: totalTransfer,
			TotalTransfers:   totalTransfer,
			CurrentRelays:    completedRelaying,
			InQueueRelays:    inQueue,
			Stage:            "relaying",
		}
		
		if err := relayerQueue.Flush(); err != nil {
			logger.Error("Failed to flush queue", zap.Error(err))
			
			progressCh <- ProgressUpdate{
				FromChain:      fromChain.GetChainID(),
				ToChain:        toChain.GetChainID(),
				TotalTransfers: totalTransfer,
				IsError:        true,
				ErrorMessage:   err.Error(),
				Stage:          "error",
			}
			
			close(progressCh)
			return
		}

		logger.Info("Queue flushed successfully",
			zap.String("from-chain", fromChain.GetChainID()),
			zap.String("to-chain", toChain.GetChainID()),
			zap.Int("completed-packets", totalTransfer))

		progressCh <- ProgressUpdate{
			FromChain:        fromChain.GetChainID(),
			ToChain:          toChain.GetChainID(),
			CurrentTransfers: totalTransfer,
			TotalTransfers:   totalTransfer,
			CurrentRelays:    totalTransfer,
			InQueueRelays:    0,
			IsCompleted:      true,
			Stage:            "completed",
		}
		
		close(progressCh)
	}()

	return progressCh, nil
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