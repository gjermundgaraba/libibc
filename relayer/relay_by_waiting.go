package relayer

import (
	"context"
	"time"

	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (rq *RelayerQueue) relayByWaiting(ctx context.Context, packets []ibc.Packet) error {
	txIDs := make([]string, len(packets))
	for i, packet := range packets {
		txIDs[i] = packet.TxHash
	}
	rq.logger.Info("Waiting for packet receipts (i.e. waiting for smart relayer to pick up packets)", zap.Int("num_packets", len(packets)), zap.Strings("tx_ids", txIDs), zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()))
	waitingPackets := make([]ibc.Packet, len(packets))
	copy(waitingPackets, packets)

	maxWait := 120 * time.Minute
	waitStart := time.Now()
	numAttempts := 0

	for len(waitingPackets) > 0 && time.Since(waitStart) < maxWait {
		if numAttempts%10 == 0 {
			txIDs := make([]string, len(packets))
			for i, packet := range packets {
				txIDs[i] = packet.TxHash
			}

			rq.logger.Info("Waiting for packet receipts", zap.Strings("tx_ids", txIDs), zap.Int("num_packets", len(waitingPackets)), zap.Duration("elapsed", time.Since(waitStart)), zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()))
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

	rq.logger.Info("Finished waiting for packet receipts", zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()), zap.Int("num_packets", len(packets)), zap.Duration("elapsed", time.Since(waitStart)))

	return nil
}
