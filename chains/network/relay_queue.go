package network

import (
	"context"
	"sync"
	"time"

	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type RelayerQueue struct {
	logger *zap.Logger

	relayer          Relayer
	selfRelay        bool
	relayMutex       sync.RWMutex
	relayerWallet    Wallet
	sourceChain      Chain
	destinationChain Chain

	// queue of packets to relay
	queue             []ibc.Packet
	queueSize         int
	queueMutex        sync.RWMutex
	currentlyRelaying int
	relaysCompleted   int

	errGroup *errgroup.Group
}

func (n *Network) NewRelayerQueue(logger *zap.Logger, sourceChain Chain, destinationChain Chain, relayerWallet Wallet, queueSize int, selfRelay bool) *RelayerQueue {
	return &RelayerQueue{
		logger: logger,

		relayer:          n.Relayer,
		selfRelay:        selfRelay,
		relayMutex:       sync.RWMutex{},
		relayerWallet:    relayerWallet,
		sourceChain:      sourceChain,
		destinationChain: destinationChain,

		queue:      make([]ibc.Packet, 0),
		queueSize:  queueSize,
		queueMutex: sync.RWMutex{},

		errGroup: &errgroup.Group{},
	}
}

func (rq *RelayerQueue) Add(packet ibc.Packet) {
	rq.queueMutex.Lock()
	defer rq.queueMutex.Unlock()

	rq.queue = append(rq.queue, packet)
	if len(rq.queue) >= rq.queueSize {
		queueCopy := make([]ibc.Packet, len(rq.queue))
		copy(queueCopy, rq.queue)

		rq.errGroup.Go(func() error {
			return rq.relay(queueCopy...)
		})
		rq.queue = make([]ibc.Packet, 0)
	}
}

func (rq *RelayerQueue) Status() (currentInQueue int, currentlyRelaying int, relayesCompleted int) {
	rq.queueMutex.RLock()
	defer rq.queueMutex.RUnlock()
	rq.relayMutex.RLock()
	defer rq.relayMutex.RUnlock()

	return len(rq.queue), rq.currentlyRelaying, rq.relaysCompleted
}

func (rq *RelayerQueue) Flush() error {
	rq.queueMutex.Lock()
	defer rq.queueMutex.Unlock()

	if len(rq.queue) > 0 {
		queueCopy := make([]ibc.Packet, len(rq.queue))
		copy(queueCopy, rq.queue)

		return rq.relay(queueCopy...)
	}

	rq.queue = make([]ibc.Packet, 0)

	return nil
}

func (rq *RelayerQueue) relay(packets ...ibc.Packet) error {
	rq.relayMutex.Lock()
	defer rq.relayMutex.Unlock()

	rq.currentlyRelaying += len(packets)

	ctx := context.Background()

	if rq.selfRelay {
		txIDs := make([]string, len(packets))
		for i, packet := range packets {
			txIDs[i] = packet.TxHash
		}
		destClient := packets[0].DestinationClient

		rq.logger.Info("Relaying packets", zap.Strings("tx_ids", txIDs), zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()), zap.String("destination_client", destClient), zap.Any("relayer-wallet", rq.relayerWallet.Address()))

		_, err := rq.relayer.Relay(ctx, rq.sourceChain, rq.destinationChain, destClient, rq.relayerWallet, txIDs)
		if err != nil {
			return errors.Wrapf(err, "failed to relay packets: %v", txIDs)
		}

		rq.logger.Info("Finished relaying packets", zap.Strings("tx_ids", txIDs), zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()), zap.String("destination_client", destClient), zap.Any("relayer-address", rq.relayerWallet.Address()))
	} else {
		// Just wait for packet receipts
		waitingPackets := make([]ibc.Packet, len(packets))
		copy(waitingPackets, packets)

		maxWait := 120 * time.Minute
		waitStart := time.Now()
		numAttempts := 0

		for len(waitingPackets) > 0 && time.Since(waitStart) < maxWait {
			if numAttempts%10 == 0 {
				rq.logger.Info("Waiting for packet receipts", zap.Int("num_packets", len(waitingPackets)), zap.Duration("elapsed", time.Since(waitStart)))
			}

			var remainingPackets []ibc.Packet
			for _, packet := range packets {
				hasPacketReceipt, err := rq.destinationChain.IsPacketReceived(ctx, packet)
				if err != nil {
					rq.logger.Debug("Failed to check packet receipt", zap.String("tx_hash", packet.TxHash), zap.Error(err))

					hasPacketReceipt = false
				}

				if !hasPacketReceipt {
					remainingPackets = append(remainingPackets, packet)
				}
			}

			time.Sleep(5 * time.Second)

			waitingPackets = remainingPackets
			numAttempts++
		}

		if len(waitingPackets) > 0 {
			return errors.Errorf("failed to relay packets: %v", waitingPackets)
		}
	}

	rq.relaysCompleted += len(packets)
	rq.currentlyRelaying -= len(packets)

	return nil
}
