package skipapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/gogoproto/proto"

	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/gjermundgaraba/libibc/chains/ethereum"
	"github.com/gjermundgaraba/libibc/chains/ethereum/erc20"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
)

type Client struct {
	baseUrl string
	logger  *zap.Logger

	cdc codec.Codec
}

func NewClient(logger *zap.Logger, baseUrl string) *Client {
	logger.Debug("creating skip api client", zap.String("baseUrl", baseUrl))
	return &Client{
		baseUrl: baseUrl,
		logger:  logger,
		cdc:     cosmos.SetupCodec(),
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

func (c *Client) GetTransferTxs(ctx context.Context, srcDenom string, fromChainID string, destDenom string, toChainID string, from string, to string, amount *big.Int) ([]network.NewTx, error) {
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
		Operations:               routeResp.Operations,
		EstimatedAmountOut:       routeResp.EstimatedAmountOut,
		SlippageTolerancePercent: "1",
		ChainIdsToAffiliates:     ChainIdsToAffiliates{},
	})
	if err != nil {
		return nil, err
	}

	// depending on msgsResp.Txs, we need to convert it to the correct byte format that can be used by the chain in question
	// The []byte needs to be in the same format that is outputed by the eureka API.
	// In other words, for cosmos, it needs to be marshalled as a txtypes.TxBody
	// And for etheruem, it needs to be the data that goes into the transaction
	var txs []network.NewTx
	for _, tx := range msgsResp.Txs {
		if tx.CosmosTx != nil && tx.EvmTx != nil {
			return nil, errors.New("both cosmos and ethereum txs are not supported")
		}

		if tx.CosmosTx != nil {
			newTx, err := cosmosTxToNewTx(c.cdc, *tx.CosmosTx)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to convert cosmos tx into new tx")
			}

			txs = append(txs, newTx)
		} else if tx.EvmTx != nil {
			newTx, err := evmTxToNewTx(*tx.EvmTx)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to convert evm tx into new tx")
			}

			txs = append(txs, newTx...)
		} else {
			return nil, errors.New("no tx type found")
		}
	}
	return txs, nil
}

func cosmosTxToNewTx(cdc codec.Codec, cosmosTx CosmosTx) (network.NewTx, error) {
	var msgs []*codectypes.Any
	for _, msg := range cosmosTx.Msgs {
		msgAny, err := cosmosMsgToAny(cdc, msg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert cosmos msg to new tx")
		}
		msgs = append(msgs, msgAny)
	}

	txBody := txtypes.TxBody{
		Messages: msgs,
	}

	txBodyBz, err := proto.Marshal(&txBody)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal tx body")
	}

	return cosmos.NewCosmosNewTx(txBodyBz), nil
}

func cosmosMsgToAny(cdc codec.Codec, msg CosmosTxMsg) (*codectypes.Any, error) {
	protoMsg, err := cdc.InterfaceRegistry().Resolve(msg.MsgTypeURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve msg type %s", msg.MsgTypeURL)
	}

	if err := cdc.UnmarshalJSON([]byte(msg.Msg), protoMsg); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal msg %s", msg.Msg)
	}

	any, err := codectypes.NewAnyWithValue(protoMsg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new Any")
	}

	return any, nil
}

func evmTxToNewTx(evmTx EvmTx) ([]network.NewTx, error) {
	var txs []network.NewTx

	for _, erc20Approval := range evmTx.RequiredErc20Approvals {
		newTx, err := requiredErc20ApprovalToNewTx(erc20Approval)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert erc20 approval to new tx")
		}
		txs = append(txs, newTx)
	}

	to := ethcommon.HexToAddress(evmTx.To)
	data, err := hex.DecodeString(evmTx.Data)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode tx data")
	}

	newTx := ethereum.NewEthNewTx(data, to)
	txs = append(txs, newTx)

	return txs, nil
}

func requiredErc20ApprovalToNewTx(approval RequiredErc20Approval) (network.NewTx, error) {
	abi, err := erc20.ContractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	spender := ethcommon.HexToAddress(approval.Spender)
	value := big.NewInt(0)
	value.SetString(approval.Amount, 10)
	data, err := abi.Pack("approve", spender, value)
	if err != nil {
		return nil, err
	}

	erc20Address := ethcommon.HexToAddress(approval.TokenContract)
	newTx := ethereum.NewEthNewTx(data, erc20Address)
	return newTx, nil
}
