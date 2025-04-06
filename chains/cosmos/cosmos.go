package cosmos

import (
	"context"
	"math/big"

	"github.com/cosmos/cosmos-sdk/codec"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var _ network.Chain = &Cosmos{}

type Cosmos struct {
	ChainID      string
	Clients      map[string]network.ClientCounterparty
	Wallets      map[string]Wallet
	Bech32Prefix string
	GasDenom     string

	grpcAddr string
	codec    codec.Codec
	logger   *zap.Logger
}

func NewCosmos(logger *zap.Logger, chainID string, bech32Prefix string, gasDenom string, grpc string) (*Cosmos, error) {
	codec := SetupCodec()
	return &Cosmos{
		ChainID:      chainID,
		Clients:      make(map[string]network.ClientCounterparty),
		Wallets:      make(map[string]Wallet),
		Bech32Prefix: bech32Prefix,
		GasDenom:     gasDenom,

		grpcAddr: grpc,
		codec:    codec,
		logger:   logger,
	}, nil
}

// GetChainID implements network.Chain.
func (c *Cosmos) GetChainID() string {
	return c.ChainID
}

// GetChainType implements network.Chain.
func (c *Cosmos) GetChainType() network.ChainType {
	return network.ChainTypeCosmos
}

// AddClient implements network.Chain.
func (c *Cosmos) AddClient(clientID string, counterparty network.ClientCounterparty) {
	c.Clients[clientID] = counterparty
}

// GetCounterpartyClient implements network.Chain.
func (c *Cosmos) GetCounterpartyClient(clientID string) (network.ClientCounterparty, error) {
	counterparty, ok := c.Clients[clientID]
	if !ok {
		return network.ClientCounterparty{}, errors.Errorf("client %s not found", clientID)
	}

	return counterparty, nil
}

// GetClients implements network.Chain.
func (c *Cosmos) GetClients() map[string]network.ClientCounterparty {
	return c.Clients
}

func (c *Cosmos) QueryTx(ctx context.Context, txHash string) (*txtypes.GetTxResponse, error) {
	grpcConn, err := utils.GetGRPC(c.grpcAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get grpc connection")
	}
	txClient := txtypes.NewServiceClient(grpcConn)
	txResponse, err := txClient.GetTx(ctx, &txtypes.GetTxRequest{Hash: txHash})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query transaction %s", txHash)
	}
	return txResponse, nil
}

// GetBalance implements network.Chain.
func (c *Cosmos) GetBalance(ctx context.Context, address string, denom string) (*big.Int, error) {
	grpcConn, err := utils.GetGRPC(c.grpcAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get grpc connection")
	}

	bankClient := banktypes.NewQueryClient(grpcConn)
	resp, err := bankClient.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: address,
		Denom:   denom,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query balance for address %s and denom %s", address, denom)
	}

	return new(big.Int).SetInt64(resp.Balance.Amount.Int64()), nil
}
