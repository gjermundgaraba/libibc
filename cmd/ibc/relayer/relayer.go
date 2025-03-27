package relayer

import (
	context "context"
	"encoding/hex"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Relayer struct {
	grpcAddr string
	logger   *zap.Logger
}

var _ network.Relayer = &Relayer{}

func NewRelayer(logger *zap.Logger, relayerGRPCAddr string) *Relayer {
	return &Relayer{
		grpcAddr: relayerGRPCAddr,
		logger:   logger,
	}
}

func (r *Relayer) Relay(ctx context.Context, srcChain network.Chain, dstChain network.Chain, srcClient string, dstClient string, relayerWallet network.Wallet, txIds []string) (string, error) {
	conn, err := utils.GetGRPC(r.grpcAddr)
	if err != nil {
		return "", errors.Wrap(err, "failed to get grpc connection")
	}

	txIdsBytes := make([][]byte, len(txIds))
	for i, txId := range txIds {
		var bz []byte
		if strings.HasPrefix(txId, "0x") {
			// Ethereum txId
			hash := ethcommon.HexToHash(txId)
			bz = hash.Bytes()
		} else {
			// Cosmos txId
			bz, err = hex.DecodeString(txId)
			if err != nil {
				return "", errors.Wrap(err, "failed to hex decode txId from cosmos")
			}
		}
		txIdsBytes[i] = bz
	}

	relayerClient := NewRelayerServiceClient(conn)

	req := &RelayByTxRequest{
		SrcChain:    srcChain.GetChainID(),
		DstChain:    dstChain.GetChainID(),
		SourceTxIds: txIdsBytes,
		SrcClientId: srcClient,
		DstClientId: dstClient,
	}
	r.logger.Debug("Starting relay request", zap.String("srcChain", srcChain.GetChainID()), zap.String("dstChain", dstChain.GetChainID()), zap.Strings("txIds", txIds), zap.Any("txIdsBytes", txIdsBytes), zap.String("targetClientId", dstClient))
	resp, err := relayerClient.RelayByTx(ctx, req)
	if err != nil {
		return "", errors.Wrapf(err, "failed to relay tx: %s, with request: %v", err, req)
	}

	r.logger.Info("Relay request successful", zap.String("srcChain", srcChain.GetChainID()), zap.String("dstChain", dstChain.GetChainID()), zap.Any("txIds", txIds), zap.String("targetClientId", dstClient))

	return dstChain.SubmitRelayTx(ctx, resp.Tx, relayerWallet)
}
