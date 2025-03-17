package network

import (
	"context"
	"maps"
	"math/big"
	"time"

	"github.com/gjermundgaraba/libibc/ibc"
	"go.uber.org/zap"
)

type Network struct {
	Relayer     Relayer
	logger      *zap.Logger
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
	GetWallets() []Wallet

	AddClient(clientID string, counterparty ClientCounterparty)
	GetClients() map[string]ClientCounterparty

	GetPackets(ctx context.Context, txHash string) ([]ibc.Packet, error)

	SubmitRelayTx(ctx context.Context, txBz []byte, walletID string) (string, error)
	SendTransfer(ctx context.Context, clientID string, walletID string, amount *big.Int, denom string, to string) (ibc.Packet, error)
}

type Wallet interface {
	GetID() string
	GetAddress() string
}

type Relayer interface {
	Relay(ctx context.Context, srcChain Chain, dstChain Chain, dstClient string, walletID string, txIds []string) (string, error)
}

func BuildNetwork(logger *zap.Logger, chains []Chain, relayer Relayer) (*Network, error) {
	network := &Network{
		Relayer:     relayer,
		logger:      logger,
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

func (n *Network) TransferWithRelay(
	ctx context.Context,
	srcChain Chain,
	dstChain Chain,
	srcClient string,
	senderWalletID string,
	srcRelayerWalletID string,
	dstRelayerWalletID string,
	amount *big.Int,
	denom string,
	to string,
) error {
	packet, err := srcChain.SendTransfer(ctx, srcClient, senderWalletID, amount, denom, to)
	if err != nil {
		return err
	}

	sendRelayTxHash, err := n.Relayer.Relay(ctx, srcChain, dstChain, packet.DestinationClient, dstRelayerWalletID, []string{packet.TxHash})
	if err != nil {
		return err
	}

	n.logger.Info("Relay send transfer tx hash", zap.String("txHash", sendRelayTxHash))

	time.Sleep(30 * time.Second)

	ackRelayTxHash, err := n.Relayer.Relay(ctx, dstChain, srcChain, srcClient, srcRelayerWalletID, []string{sendRelayTxHash})
	if err != nil {
		return err
	}

	n.logger.Info("Relay ack tx hash", zap.String("txHash", ackRelayTxHash))

	return nil
}

// func (n *Network) TracePacket(packet ibc.Packet) error {
//
// }
