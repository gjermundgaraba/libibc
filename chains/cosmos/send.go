package cosmos

import (
	"context"
	"math/big"

	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (c *Cosmos) Send(ctx context.Context, senderWallet network.Wallet, amount *big.Int, denom string, toAddress string) (string, error) {
	cosmosWallet, ok := senderWallet.(*Wallet)
	if !ok {
		return "", errors.Errorf("invalid wallet type: %T", senderWallet)
	}
	fromAddress := senderWallet.Address()

	grpcConn, err := utils.GetGRPC(c.grpcAddr)
	if err != nil {
		return "", errors.Wrap(err, "failed to get gRPC connection")
	}

	accountClient := accounttypes.NewQueryClient(grpcConn)
	accountRes, err := accountClient.AccountInfo(ctx, &accounttypes.QueryAccountInfoRequest{Address: fromAddress})
	if err != nil {
		return "", errors.Wrap(err, "failed to get account info")
	}

	amountCoin := sdk.NewInt64Coin(denom, amount.Int64())
	sendMsg := banktypes.NewMsgSend(
		sdk.MustAccAddressFromBech32(fromAddress),
		sdk.MustAccAddressFromBech32(toAddress),
		sdk.NewCoins(amountCoin),
	)

	txCfg := authtx.NewTxConfig(c.codec, authtx.DefaultSignModes)
	txBuilder := txCfg.NewTxBuilder()
	txBuilder.SetGasLimit(200000)
	// TODO: Make fee and denom part of the config!
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uatom", 200000)))
	txBuilder.SetMsgs(sendMsg)

	sigV2 := signing.SignatureV2{
		PubKey: cosmosWallet.privateKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(txCfg.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
		Sequence: accountRes.Info.Sequence,
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return "", errors.Wrap(err, "failed to set initial signature")
	}
	signerData := xauthsigning.SignerData{
		Address:       fromAddress,
		ChainID:       c.ChainID,
		AccountNumber: accountRes.Info.AccountNumber,
	}

	sigV2, err = tx.SignWithPrivKey(
		ctx,
		signing.SignMode(txCfg.SignModeHandler().DefaultMode()),
		signerData,
		txBuilder,
		cosmosWallet.privateKey,
		txCfg,
		accountRes.Info.Sequence,
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign with private key")
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return "", errors.Wrap(err, "failed to set final signature")
	}

	txBytes, err := txCfg.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return "", errors.Wrap(err, "failed to encode transaction")
	}

	txClient := txtypes.NewServiceClient(grpcConn)
	grpcRes, err := txClient.BroadcastTx(
		ctx,
		&txtypes.BroadcastTxRequest{
			Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes,
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to broadcast transaction")
	}

	if grpcRes.TxResponse.Code != 0 {
		return "", errors.Errorf("transaction failed with code %d: %s", grpcRes.TxResponse.Code, grpcRes.TxResponse.RawLog)
	}

	c.logger.Info("Send transaction broadcasted", zap.String("tx_hash", grpcRes.TxResponse.TxHash), zap.String("from", fromAddress), zap.String("to", toAddress), zap.String("amount", amount.String()), zap.String("denom", denom))

	return grpcRes.TxResponse.TxHash, nil
}
