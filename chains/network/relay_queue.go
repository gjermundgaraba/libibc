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
	relayMutex       sync.Mutex
	relayerWallet    Wallet
	sourceChain      Chain
	destinationChain Chain

	// queue of packets to relay
	queue      []ibc.Packet
	queueSize  int
	queueMutex sync.Mutex

	errGroup *errgroup.Group
}

func (n *Network) NewRelayerQueue(logger *zap.Logger, sourceChain Chain, destinationChain Chain, relayerWallet Wallet, queueSize int) *RelayerQueue {
	return &RelayerQueue{
		logger: logger,

		relayer:          n.Relayer,
		relayMutex:       sync.Mutex{},
		relayerWallet:    relayerWallet,
		sourceChain:      sourceChain,
		destinationChain: destinationChain,

		queue:      make([]ibc.Packet, 0),
		queueSize:  queueSize,
		queueMutex: sync.Mutex{},

		errGroup: &errgroup.Group{},
	}
}

func (rq *RelayerQueue) Add(packet ibc.Packet) {
	rq.queueMutex.Lock()
	defer rq.queueMutex.Unlock()

	rq.queue = append(rq.queue, packet)
	if len(rq.queue) >= rq.queueSize {
		rq.errGroup.Go(func() error {
			return rq.relay(rq.queue...)
		})
		rq.queue = make([]ibc.Packet, 0)
	}
}

func (rq *RelayerQueue) Flush() error {
	rq.queueMutex.Lock()
	defer func() {
		rq.queue = make([]ibc.Packet, 0)
		rq.queueMutex.Unlock()
	}()

	if len(rq.queue) > 0 {
		rq.errGroup.Go(func() error {
			return rq.relay(rq.queue...)
		})
	}

	return rq.errGroup.Wait()
}

func (rq *RelayerQueue) relay(packets ...ibc.Packet) error {
	rq.relayMutex.Lock()
	defer rq.queueMutex.Unlock()

	ctx := context.Background()

	txIDs := make([]string, len(packets))
	for i, packet := range packets {
		txIDs[i] = packet.TxHash
	}
	destClient := packets[0].DestinationClient

	rq.logger.Info("relaying packets", zap.Strings("tx_ids", txIDs), zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()), zap.String("destination_client", destClient), zap.Any("relayer_wallet", rq.relayerWallet))
	time.Sleep(30 * time.Second)

	_, err := rq.relayer.Relay(ctx, rq.sourceChain, rq.destinationChain, destClient, rq.relayerWallet, txIDs)
	if err != nil {
		return errors.Wrapf(err, "failed to relay packets: %v", txIDs)
	}

	return nil
}
