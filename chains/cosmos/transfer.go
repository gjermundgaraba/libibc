package cosmos

import (
	"context"
	"math/big"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SendTransfer implements network.Chain.
func (c *Cosmos) SendTransfer(
	ctx context.Context,
	clientID string,
	wallet network.Wallet,
	amount *big.Int,
	denom string,
	to string,
) (ibc.Packet, error) {
	cosmosWallet, ok := wallet.(*Wallet)
	if !ok {
		return ibc.Packet{}, errors.Errorf("invalid wallet type: %T", wallet)
	}

	timeout := uint64(time.Now().Add(6 * time.Hour).Unix())
	transferCoin := sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(amount))

	transferPayload := transfertypes.FungibleTokenPacketData{
		Denom:    transferCoin.Denom,
		Amount:   transferCoin.Amount.String(),
		Sender:   wallet.Address(),
		Receiver: to,
		Memo:     "", // TODO: add memo for load testing purposed
	}
	encodedPayload, err := transfertypes.EncodeABIFungibleTokenPacketData(&transferPayload)
	if err != nil {
		return ibc.Packet{}, errors.Wrap(err, "failed to encode transfer payload")
	}

	payload := channeltypesv2.Payload{
		SourcePort:      transfertypes.PortID,
		DestinationPort: transfertypes.PortID,
		Version:         transfertypes.V1,
		Encoding:        transfertypes.EncodingABI,
		Value:           encodedPayload,
	}
	msgSendPacket := channeltypesv2.MsgSendPacket{
		SourceClient:     clientID,
		TimeoutTimestamp: timeout,
		Payloads: []channeltypesv2.Payload{
			payload,
		},
		Signer: wallet.Address(),
	}

	resp, err := c.submitTx(ctx, cosmosWallet, &msgSendPacket)
	if err != nil {
		return ibc.Packet{}, errors.Wrap(err, "failed to submit tx")
	}

	packets, err := c.GetPackets(ctx, resp.TxResponse.TxHash)
	if err != nil {
		return ibc.Packet{}, errors.Wrapf(err, "failed to get packets for transfer with tx hash: %s", resp.TxResponse.TxHash)
	}
	if len(packets) != 1 {
		return ibc.Packet{}, errors.Errorf("failed to get packet for transfer (expected 1, got %d)", len(packets))
	}

	c.logger.Info("Sent transfer", zap.String("tx_hash", resp.TxResponse.TxHash), zap.String("from", wallet.Address()), zap.String("to", to), zap.Uint64("amount", amount.Uint64()), zap.String("denom", denom))

	return packets[0], nil
}
