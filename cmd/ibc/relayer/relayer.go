package relayer

import (
	context "context"

	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
)

type Relayer struct {
	grpcAddr string
}

func NewRelayer(relayerGRPCAddr string) *Relayer {
	return &Relayer{
		grpcAddr: relayerGRPCAddr,
	}
}

func (r *Relayer) Relay(ctx context.Context, srcChain network.Chain, dstChain network.Chain, dstClient string, walletID string, txIds [][]byte) (string, error) {
	conn, err := utils.GetGRPC(r.grpcAddr)
	if err != nil {
		return "", errors.Wrap(err, "failed to get grpc connection")
	}

	relayerClient := NewRelayerServiceClient(conn)

	req := &RelayByTxRequest{
		SrcChain:       srcChain.GetChainID(),
		DstChain:       dstChain.GetChainID(),
		SourceTxIds:    txIds,
		TargetClientId: dstClient,
	}
	resp, err := relayerClient.RelayByTx(ctx, req)
	if err != nil {
		return "", errors.Wrapf(err, "failed to relay tx: %s, with request: %v", err, req)
	}

	return dstChain.SubmitRelayTx(ctx, resp.Tx, walletID)
}
