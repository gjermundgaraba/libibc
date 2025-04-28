package cosmos

import (
	"context"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/gjermundgaraba/libibc/chainclients/network"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (c *Cosmos) Send(ctx context.Context, senderWallet network.Wallet, amount *big.Int, denom string, toAddress string) (string, error) {
	c.logger.Debug("sending bank transfer", zap.String("sender wallet id", senderWallet.ID()), zap.String("sender address", senderWallet.Address()), zap.String("to", toAddress), zap.String("amount", amount.String()), zap.String("denom", denom))

	cosmosWallet, ok := senderWallet.(*Wallet)
	if !ok {
		return "", errors.Errorf("invalid wallet type: %T", senderWallet)
	}
	fromAddress := senderWallet.Address()

	amountCoin := sdk.NewInt64Coin(denom, amount.Int64())
	sendMsg := banktypes.NewMsgSend(
		sdk.MustAccAddressFromBech32(fromAddress),
		sdk.MustAccAddressFromBech32(toAddress),
		sdk.NewCoins(amountCoin),
	)

	resp, err := c.submitTx(ctx, cosmosWallet, 200_000, sendMsg)
	if err != nil {
		return "", errors.Wrap(err, "failed to submit bank send tx")
	}

	return resp.TxHash, nil
}
