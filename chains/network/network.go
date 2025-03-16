package network

import (
	"context"
	"maps"
	"math/big"

	"github.com/gjermundgaraba/libibc/ibc"
)

type Network struct {
	Relayer     Relayer
	chains      map[string]Chain
	connections map[string]ClientCounterparty
}

type ClientCounterparty struct {
	ClientID string
	ChainID  string
}

type Chain interface {
	GetChainID() string

	AddWallet(walletID string, privateKeyHex string) error
	GetWallet(walletID string) Wallet

	AddClient(clientID string, counterparty ClientCounterparty)
	GetClients() map[string]ClientCounterparty

	GetPackets(ctx context.Context, txHash string) ([]ibc.Packet, error)

	SubmitRelayTx(ctx context.Context, txBz []byte, walletID string) (string, error)
	SendTransfer(ctx context.Context, clientID string, walletID string, amount *big.Int, denom string, to string) (ibc.Packet, error)
}

type Wallet interface {
	GetAddress() string
}

type Relayer interface {
	Relay(ctx context.Context, srcChain Chain, dstChain Chain, dstClient string, walletID string, txIds [][]byte) (string, error)
}

func BuildNetwork(chains []Chain, relayer Relayer) (*Network, error) {
	network := &Network{
		Relayer:     relayer,
		chains:      make(map[string]Chain),
		connections: make(map[string]ClientCounterparty),
	}

	for _, chain := range chains {
		network.chains[chain.GetChainID()] = chain

		maps.Copy(network.connections, chain.GetClients())
	}

	return network, nil
}

func (n *Network) GetChain(chainID string) Chain {
	return n.chains[chainID]
}

// func (n *Network) TracePacket(packet ibc.Packet) error {
//
// }
