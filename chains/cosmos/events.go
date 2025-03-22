package cosmos

import (
	"encoding/hex"
	"strconv"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
)

func ParsePackets(txHash string, events []abci.Event) ([]ibc.Packet, error) {
	ibcVersion, err := determineIBCVersion(events)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to determine IBC version")
	}

	switch ibcVersion {
	case 1:
		v1Packets, err := parseIBCV1Packets(events)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse IBC v1 packets")
		}

		var packets []ibc.Packet
		for _, packet := range v1Packets {
			packets = append(packets, ibc.NewPacket(
				txHash,
				1,
				packet.Sequence,
				packet.SourcePort,
				packet.DestinationPort,
				packet.TimeoutTimestamp,
				packet,
			))
		}
		return packets, nil
	case 2:
		v2Packets, err := parseIBCV2Packet(events)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse IBC v2 packets")
		}

		var packets []ibc.Packet
		for _, packet := range v2Packets {
			packets = append(packets, ibc.NewPacket(
				txHash,
				2,
				packet.Sequence,
				packet.SourceClient,
				packet.DestinationClient,
				packet.TimeoutTimestamp,
				packet,
			))
		}
		return packets, nil
	default:
		return nil, errors.New("unknown IBC version")
	}
}

func determineIBCVersion(events []abci.Event) (uint, error) {
	for _, event := range events {
		if event.Type == "message" {
			for _, attribute := range event.Attributes {
				if attribute.Key == "module" && attribute.Value == "ibc_channel" {
					return 1, nil
				} else if attribute.Key == "module" && attribute.Value == "ibc_channelv2" {
					return 2, nil
				}
			}
		}
	}

	return 0, errors.New("could not determine IBC version")
}

func parseIBCV2Packet(events []abci.Event) ([]channeltypesv2.Packet, error) {
	var packets []channeltypesv2.Packet
	for _, event := range events {
		if event.Type == "send_packet" {
			for _, attribute := range event.Attributes {
				if attribute.Key == "encoded_packet_hex" {
					data, err := hex.DecodeString(attribute.Value)
					if err != nil {
						return nil, errors.Wrap(err, "failed to decode encoded_packet_hex string")
					}

					var ibcPacket channeltypesv2.Packet
					if err := proto.Unmarshal(data, &ibcPacket); err != nil {
						return nil, errors.Wrap(err, "failed to unmarshal IBC packet")
					}

					packets = append(packets, ibcPacket)
				}
			}
		}
	}

	if len(packets) == 0 {
		return nil, errors.New("no IBC v2 packets found in events")
	}

	return packets, nil
}

// ParsePacketsFromEvents parses events emitted from a MsgRecvPacket and returns
// all the packets found.
// Returns an error if no packet is found.
func parseIBCV1Packets(events []abci.Event) ([]channeltypes.Packet, error) {
	var packets []channeltypes.Packet
	for _, ev := range events {
		if ev.Type == "send_packet" {
			var packet channeltypes.Packet
			for _, attr := range ev.Attributes {
				switch attr.Key {
				case channeltypes.AttributeKeyDataHex:
					data, err := hex.DecodeString(attr.Value)
					if err != nil {
						return nil, errors.Wrap(err, "failed to decode data hex string")
					}
					packet.Data = data
				case channeltypes.AttributeKeySequence:
					seq, err := strconv.ParseUint(attr.Value, 10, 64)
					if err != nil {
						return nil, errors.Wrap(err, "failed to parse sequence")
					}

					packet.Sequence = seq

				case channeltypes.AttributeKeySrcPort:
					packet.SourcePort = attr.Value

				case channeltypes.AttributeKeySrcChannel:
					packet.SourceChannel = attr.Value

				case channeltypes.AttributeKeyDstPort:
					packet.DestinationPort = attr.Value

				case channeltypes.AttributeKeyDstChannel:
					packet.DestinationChannel = attr.Value

				case channeltypes.AttributeKeyTimeoutHeight:
					height, err := clienttypes.ParseHeight(attr.Value)
					if err != nil {
						return nil, errors.Wrap(err, "failed to parse height")
					}

					packet.TimeoutHeight = height

				case channeltypes.AttributeKeyTimeoutTimestamp:
					timestamp, err := strconv.ParseUint(attr.Value, 10, 64)
					if err != nil {
						return nil, errors.Wrap(err, "failed to parse timestamp")
					}

					packet.TimeoutTimestamp = timestamp

				default:
					continue
				}
			}

			packets = append(packets, packet)
		}
	}
	if len(packets) == 0 {
		return nil, errors.New("no packets found in events")
	}

	return packets, nil
}
