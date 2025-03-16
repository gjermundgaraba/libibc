package cosmos

import (
	"context"

	"github.com/cosmos/cosmos-sdk/codec"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
)

var _ network.Chain = &Cosmos{}

type Cosmos struct {
	ChainID string
	Clients map[string]network.ClientCounterparty
	Wallets map[string]Wallet

	grpcAddr string
	codec    codec.Codec
}

func NewCosmos(chainID string, grpc string) (*Cosmos, error) {
	codec := SetupCodec()
	return &Cosmos{
		ChainID: chainID,
		Clients: make(map[string]network.ClientCounterparty),
		Wallets: make(map[string]Wallet),

		grpcAddr: grpc,
		codec:    codec,
	}, nil
}

// GetChainID implements network.Chain.
func (c *Cosmos) GetChainID() string {
	return c.ChainID
}

func (c *Cosmos) GetWallet(walletID string) network.Wallet {
	wallet := c.Wallets[walletID]
	return &wallet
}

func (c *Cosmos) AddClient(clientID string, counterparty network.ClientCounterparty) {
	c.Clients[clientID] = counterparty
}

// GetClients implements network.Chain.
func (c *Cosmos) GetClients() map[string]network.ClientCounterparty {
	return c.Clients
}

func (c *Cosmos) GetPackets(ctx context.Context, txHash string) ([]ibc.Packet, error) {
	txResp, err := c.QueryTx(ctx, txHash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query transaction %s", txHash)
	}

	events := txResp.TxResponse.Events
	return ParsePackets(txHash, events)
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
