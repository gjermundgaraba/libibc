package eurekarelayerapi

import (
	context "context"
	"encoding/hex"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Client struct {
	grpcAddr string
	logger   *zap.Logger
}

func NewClient(logger *zap.Logger, relayerGRPCAddr string) *Client {
	return &Client{
		grpcAddr: relayerGRPCAddr,
		logger:   logger,
	}
}

func (r *Client) RelayByTx(ctx context.Context, srcChainID string, dstChainID string, srcClientID string, dstClientID string, txIds []string) (*RelayByTxResponse, error) {
	conn, err := utils.GetGRPC(r.grpcAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get grpc connection")
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
				return nil, errors.Wrap(err, "failed to hex decode txId from cosmos")
			}
		}
		txIdsBytes[i] = bz
	}

	relayerClient := NewRelayerServiceClient(conn)

	req := &RelayByTxRequest{
		SrcChain:    srcChainID,
		DstChain:    dstChainID,
		SourceTxIds: txIdsBytes,
		SrcClientId: srcClientID,
		DstClientId: dstClientID,
	}
	r.logger.Debug("Starting relayByTx request", zap.String("srcChainID", srcChainID), zap.String("dstChainID", dstChainID), zap.String("srdClientID", srcClientID), zap.String("targetClientId", dstClientID), zap.Strings("txIds", txIds), zap.Any("txIdsBytes", txIdsBytes))
	resp, err := relayerClient.RelayByTx(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to relay tx: %s, with request: %v", err, req)
	}

	r.logger.Info("Relay request successful", zap.Any("resp", resp), zap.String("srcChainID", srcChainID), zap.String("dstChainID", dstChainID), zap.String("srdClientID", srcClientID), zap.String("targetClientId", dstClientID), zap.Strings("txIds", txIds))

	return resp, nil
}

func (r *Client) CreateClient(ctx context.Context, srcChainID string, dstChainID string, params map[string]string) (*CreateClientResponse, error) {
	conn, err := utils.GetGRPC(r.grpcAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get grpc connection")
	}

	relayerClient := NewRelayerServiceClient(conn)

	req := &CreateClientRequest{
		SrcChain:   srcChainID,
		DstChain:   dstChainID,
		Parameters: params,
	}

	r.logger.Debug("Starting create client request", zap.String("srcChainID", srcChainID), zap.String("dstChainID", dstChainID), zap.Any("params", params))
	resp, err := relayerClient.CreateClient(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed create client req: %s, with request: %v", err, req)
	}

	return resp, nil
}
