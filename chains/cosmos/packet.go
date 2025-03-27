package cosmos

import (
	"context"

	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (c *Cosmos) GetPackets(ctx context.Context, txHash string) ([]ibc.Packet, error) {
	txResp, err := c.QueryTx(ctx, txHash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query transaction %s", txHash)
	}

	events := txResp.TxResponse.Events
	return ParsePackets(txHash, events)
}

func (c *Cosmos) IsPacketReceived(ctx context.Context, packet ibc.Packet) (bool, error) {
	grpcConn, err := utils.GetGRPC(c.grpcAddr)
	if err != nil {
		return false, errors.Wrap(err, "failed to get grpc connection")
	}

	channelClient := channeltypesv2.NewQueryClient(grpcConn)
	resp, err := channelClient.PacketReceipt(ctx, &channeltypesv2.QueryPacketReceiptRequest{
		ClientId: packet.DestinationClient,
		Sequence: packet.Sequence,
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to query packet receipt")
	}
	c.logger.Debug("Querying packet receipt", zap.String("ClientID", packet.DestinationClient), zap.Uint64("Sequence", packet.Sequence), zap.Any("Response", resp))

	return resp.Received, nil
}
