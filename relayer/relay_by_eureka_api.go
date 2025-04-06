package relayer

import (
	"context"

	"github.com/gjermundgaraba/libibc/apis/eurekarelayerapi"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (rq *RelayerQueue) relayByEurekaAPI(ctx context.Context, packets []ibc.Packet) error {
	eurekaAPIClient := eurekarelayerapi.NewClient(rq.logger, rq.eurekaAPIAddr)

	txIDs := make([]string, len(packets))
	for i, packet := range packets {
		txIDs[i] = packet.TxHash
	}
	srcClientID := packets[0].SourceClient
	dstClientID := packets[0].DestinationClient

	rq.logger.Info("Relaying packets with eureka API", zap.Strings("tx_ids", txIDs), zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()), zap.String("destination_client", dstClientID), zap.Any("relayer-wallet", rq.relayerWallet.Address()))

	resp, err := eurekaAPIClient.RelayByTx(ctx, rq.sourceChain.GetChainID(), rq.destinationChain.GetChainID(), srcClientID, dstClientID, txIDs)
	if err != nil {
		return errors.Wrapf(err, "failed to call eureka relayer api RelayByTx from chain ID %s (srcClientID: %s) to chain ID %s (dstClientID: %s) with packets: %v", rq.sourceChain.GetChainID(), srcClientID, rq.destinationChain.GetChainID(), dstClientID, txIDs)
	}
	rq.logger.Debug("RelayByTx response", zap.Any("resp", resp))

	rq.destinationChain.SubmitTx(ctx, resp.Tx, rq.relayerWallet)

	rq.logger.Info("Finished relaying packets", zap.Strings("tx_ids", txIDs), zap.String("source_chain", rq.sourceChain.GetChainID()), zap.String("destination_chain", rq.destinationChain.GetChainID()), zap.String("destination_client", dstClientID), zap.Any("relayer-address", rq.relayerWallet.Address()))

	return nil
}
