package skipapi

import (
	"context"
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/gjermundgaraba/libibc/utils"
)

type Client struct {
	baseUrl string
	logger  *zap.Logger
}

func NewClient(logger *zap.Logger, baseUrl string) *Client {
	logger.Debug("creating skip api client", zap.String("baseUrl", baseUrl))
	return &Client{
		baseUrl: baseUrl,
		logger:  logger,
	}
}

func (c *Client) Route(ctx context.Context, req RouteRequest) (*RouteResponse, error) {
	url := fmt.Sprintf("%s/v2/fungible/route", c.baseUrl)
	resp, err := utils.HttpRequest[RouteResponse](ctx, c.logger, url, "POST", req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call Route API")
	}

	return &resp, nil
}

func (c *Client) Msgs(ctx context.Context, req MsgsRequest) (*MsgsResponse, error) {
	url := fmt.Sprintf("%s/v2/fungible/msgs", c.baseUrl)
	resp, err := utils.HttpRequest[MsgsResponse](ctx, c.logger, url, "POST", req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call Msgs API")
	}

	return &resp, nil
}

func (c *Client) GetTransferTxs(ctx context.Context, srcDenom string, fromChainID string, destDenom string, toChainID string, from string, to string, amount *big.Int) ([]byte, error) {
	amountStr := amount.String()
	routeResp, err := c.Route(ctx, RouteRequest{
		SourceAssetDenom:   srcDenom,
		SourceAssetChainID: fromChainID,
		DestAssetDenom:     destDenom,
		DestAssetChainID:   toChainID,
		AllowUnsafe:        true,
		ExperimentalFeatures: []string{
			"stargate", "eureka", "hyperlane",
		},
		AllowMultiTx: true,
		SmartRelay:   true,
		SmartSwapOptions: SmartSwapOptions{
			SplitRoutes: true,
			EvmSwaps:    true,
		},
		GoFast:   true,
		AmountIn: amountStr,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get route")
	}

	msgsResp, err := c.Msgs(ctx, MsgsRequest{
		SourceAssetDenom:   srcDenom,
		SourceAssetChainID: fromChainID,
		DestAssetDenom:     destDenom,
		DestAssetChainID:   toChainID,
		AmountIn:           amountStr,
		AmountOut:          routeResp.EstimatedAmountOut,
		AddressList: []string{
			from,
			to,
		},
		Operations: routeResp.Operations,
	})
	if err != nil {
		return nil, err
	}

	// TODO: depending on msgsResp.Txs, we need to convert it to the correct byte format that can be used by the chain in question
	// The []byte needs to be in the same format that is outputed by the eureka API.
	// In other words, for cosmos, it needs to be marshalled as a txtypes.TxBody
	// And for etheruem, it needs to be the data that goes into the transaction

	_ = msgsResp
	return nil, nil
}
