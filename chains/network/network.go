package network

import (
	"context"
	"maps"
	"math/big"

	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Network struct {
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
	GetCounterpartyClient(clientID string) (ClientCounterparty, error)
	GetClients() map[string]ClientCounterparty

	GetPackets(ctx context.Context, txHash string) ([]ibc.Packet, error)
	IsPacketReceived(ctx context.Context, packet ibc.Packet) (bool, error)

	SubmitTx(ctx context.Context, txBz []byte, wallet Wallet) (string, error)
	SendTransfer(ctx context.Context, clientID string, wallet Wallet, amount *big.Int, denom string, to string, memo string) (ibc.Packet, error)
	Send(ctx context.Context, wallet Wallet, amount *big.Int, denom string, toAddress string) (string, error)
	GetBalance(ctx context.Context, address string, denom string) (*big.Int, error)
}

type Wallet interface {
	ID() string
	Address() string
	PrivateKeyHex() string
}

type RelayMethod int

func BuildNetwork(logger *zap.Logger, chains []Chain) (*Network, error) {
	network := &Network{
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

func (n *Network) GetChain(chainID string) (Chain, error) {
	chain, ok := n.chains[chainID]
	if !ok || chain == nil {
		return nil, errors.Errorf("chain not found: %s", chainID)
	}

	return chain, nil
}

// func (n *Network) TransferWithRelay(
// 	ctx context.Context,
// 	relayer *relayer.RelayerQueue,
// 	srcChain Chain,
// 	dstChain Chain,
// 	srcClient string,
// 	senderWallet Wallet,
// 	srcRelayerWallet Wallet,
// 	dstRelayerWallet Wallet,
// 	amount *big.Int,
// 	denom string,
// 	to string,
// 	memo string,
// ) error {
// 	packet, err := srcChain.SendTransfer(ctx, srcClient, senderWallet, amount, denom, to, memo)
// 	if err != nil {
// 		return err
// 	}
//
// 	sendRelayTxHash, err := relayer.Relay(ctx, srcChain, dstChain, srcClient, packet.DestinationClient, dstRelayerWallet, []string{packet.TxHash})
// 	if err != nil {
// 		return err
// 	}
//
// 	n.logger.Info("Relay send transfer tx hash", zap.String("txHash", sendRelayTxHash))
//
// 	time.Sleep(30 * time.Second)
//
// 	ackRelayTxHash, err := relayer.Relay(ctx, dstChain, srcChain, packet.DestinationClient, srcClient, srcRelayerWallet, []string{sendRelayTxHash})
// 	if err != nil {
// 		return err
// 	}
//
// 	n.logger.Info("Relay ack tx hash", zap.String("txHash", ackRelayTxHash))
//
// 	return nil
// }
//
// // func (n *Network) TracePacket(packet ibc.Packet) error {
// //
// // }
