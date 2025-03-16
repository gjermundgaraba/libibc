package ibc

import (
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	"github.com/pkg/errors"
)

type Packet struct {
	TxHash            string
	IBCVersion        uint
	SourceClient      string
	DestinationClient string
	Sequence          uint64

	PacketRaw any
}

func NewPacket(txHash string, ibcVersion uint, sourceClient, destinationClient string, data any) Packet {
	return Packet{
		TxHash:            txHash,
		IBCVersion:        ibcVersion,
		SourceClient:      sourceClient,
		DestinationClient: destinationClient,
		PacketRaw:         data,
	}
}

func (p Packet) GetTransferData() (transfertypes.InternalTransferRepresentation, error) {
	var packetDataBz []byte
	encoding := transfertypes.EncodingJSON
	switch p.IBCVersion {
	case 1:
		v1Packet := p.PacketRaw.(channeltypes.Packet)
		packetDataBz = v1Packet.Data
	case 2:
		v2Packet := p.PacketRaw.(channeltypesv2.Packet)
		packetDataBz = v2Packet.Payloads[0].Value
		encoding = v2Packet.Payloads[0].Encoding
	}

	transferData, err := transfertypes.UnmarshalPacketData(packetDataBz, transfertypes.V1, encoding)
	if err != nil {
		return transfertypes.InternalTransferRepresentation{}, errors.Wrap(err, "failed to unmarshal packet data")
	}

	return transferData, nil
}
