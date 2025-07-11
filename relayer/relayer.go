package relayer

import (
	"context"
	"sync"

	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type RelayMethod string

type RelayerQueue struct {
	logger *zap.Logger

	relayMethod      RelayMethod
	relayMutex       sync.RWMutex
	relayerWallet    network.Wallet
	sourceChain      network.Chain
	destinationChain network.Chain

	selfRelay bool
	// if selfRelay is true, this needs to be set
	eurekaAPIAddr string

	// queue of packets to relay
	queue             []ibc.Packet
	queueSize         int
	queueMutex        sync.RWMutex
	currentlyRelaying int
	relaysCompleted   int

	errGroup *errgroup.Group
}

func NewRelayerQueue(logger *zap.Logger, sourceChain network.Chain, destinationChain network.Chain, relayerWallet network.Wallet, queueSize int, selfRelay bool, eurekaAPIAddr string) *RelayerQueue {

	return &RelayerQueue{
		logger: logger,

		relayMutex:       sync.RWMutex{},
		relayerWallet:    relayerWallet,
		sourceChain:      sourceChain,
		destinationChain: destinationChain,

		selfRelay:     selfRelay,
		eurekaAPIAddr: eurekaAPIAddr,

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

		if err := rq.relay(queueCopy...); err != nil {
			return errors.Wrap(err, "failed to relay packets")
		}
	} else {
		if err := rq.errGroup.Wait(); err != nil {
			return errors.Wrap(err, "failed to wait for relay")
		}
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
		if err := rq.relayByEurekaAPI(ctx, packets); err != nil {
			return errors.Wrap(err, "failed to relay packets by eureka API")
		}
	} else {
		if err := rq.relayByWaiting(ctx, packets); err != nil {
			return errors.Wrap(err, "failed to relay packets by waiting")
		}
	}

	rq.relaysCompleted += len(packets)
	rq.currentlyRelaying -= len(packets)

	return nil
}
