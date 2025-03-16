package ethereum

import (
	"context"

	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	"github.com/cosmos/solidity-ibc-eureka/abigen/ics26router"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
)

// GetPackets implements network.Chain.
func (e *Ethereum) GetPackets(ctx context.Context, txHash string) ([]ibc.Packet, error) {
	ethClient, err := ethclient.Dial(e.ethRPC)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial ethereum client")
	}

	ics26Contract, err := ics26router.NewContract(e.ics26Address, ethClient)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ics26 contract")
	}

	receipt, err := ethClient.TransactionReceipt(ctx, ethcommon.HexToHash(txHash))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction receipt")
	}

	sendPacketEvent, err := GetEvmEvent(receipt, ics26Contract.ParseSendPacket)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get write acknowledgement event")
	}
	if sendPacketEvent == nil {
		return nil, errors.New("write acknowledgement event not found")
	}

	var payloads []channeltypesv2.Payload
	for _, payload := range sendPacketEvent.Packet.Payloads {
		payloads = append(payloads, channeltypesv2.Payload{
			SourcePort:      payload.SourcePort,
			DestinationPort: payload.DestPort,
			Version:         payload.Version,
			Encoding:        payload.Encoding,
			Value:           payload.Value,
		})
	}

	packetData := channeltypesv2.Packet{
		Sequence:          sendPacketEvent.Packet.Sequence,
		SourceClient:      sendPacketEvent.Packet.SourceClient,
		DestinationClient: sendPacketEvent.Packet.DestClient,
		TimeoutTimestamp:  sendPacketEvent.Packet.TimeoutTimestamp,
		Payloads:          payloads,
	}

	packet := ibc.NewPacket(txHash, 2, sendPacketEvent.Packet.SourceClient, sendPacketEvent.Packet.DestClient, packetData)

	return []ibc.Packet{packet}, nil
}
