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
	GetWallet(walletID string) (Wallet, error)
	GetWallets() []Wallet
	GenerateWallet(walletID string) (Wallet, error)

	AddClient(clientID string, counterparty ClientCounterparty)
	GetClients() map[string]ClientCounterparty

	GetPackets(ctx context.Context, txHash string) ([]ibc.Packet, error)
	IsPacketReceived(ctx context.Context, packet ibc.Packet) (bool, error)

	SubmitRelayTx(ctx context.Context, txBz []byte, wallet Wallet) (string, error)
	SendTransfer(ctx context.Context, clientID string, wallet Wallet, amount *big.Int, denom string, to string) (ibc.Packet, error)
	Send(ctx context.Context, wallet Wallet, amount *big.Int, denom string, toAddress string) (string, error)
	GetBalance(ctx context.Context, address string, denom string) (*big.Int, error)
}

type Wallet interface {
	ID() string
	Address() string
	PrivateKeyHex() string
}

type Relayer interface {
	Relay(ctx context.Context, srcChain Chain, dstChain Chain, dstClient string, relayerWallet Wallet, txIds []string) (string, error)
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
	senderWallet Wallet,
	srcRelayerWallet Wallet,
	dstRelayerWallet Wallet,
	amount *big.Int,
	denom string,
	to string,
) error {
	packet, err := srcChain.SendTransfer(ctx, srcClient, senderWallet, amount, denom, to)
	if err != nil {
		return err
	}

	sendRelayTxHash, err := n.Relayer.Relay(ctx, srcChain, dstChain, packet.DestinationClient, dstRelayerWallet, []string{packet.TxHash})
	if err != nil {
		return err
	}

	n.logger.Info("Relay send transfer tx hash", zap.String("txHash", sendRelayTxHash))

	time.Sleep(30 * time.Second)

	ackRelayTxHash, err := n.Relayer.Relay(ctx, dstChain, srcChain, srcClient, srcRelayerWallet, []string{sendRelayTxHash})
	if err != nil {
		return err
	}

	n.logger.Info("Relay ack tx hash", zap.String("txHash", ackRelayTxHash))

	return nil
}

// func (n *Network) TracePacket(packet ibc.Packet) error {
//
// }
