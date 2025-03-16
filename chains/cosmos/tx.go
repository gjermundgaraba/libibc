package cosmos

import (
	"context"

	// dbm "github.com/cosmos/cosmos-db"
	// "github.com/cosmos/cosmos-sdk/client/tx"
	// simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
)

// SubmitTx implements network.Chain.
func (c *Cosmos) SubmitRelayTx(ctx context.Context, txBz []byte, walletID string) (string, error) {
	// Extract messages from the response (cosmos specific)
	var txBody txtypes.TxBody
	if err := proto.Unmarshal(txBz, &txBody); err != nil {
		return "", err
	}

	if len(txBody.Messages) == 0 {
		return "", errors.New("no messages in tx")
	}

	var msgs []sdk.Msg
	for _, msg := range txBody.Messages {
		var sdkMsg sdk.Msg
		if err := c.codec.InterfaceRegistry().UnpackAny(msg, &sdkMsg); err != nil {
			return "", err
		}

		msgs = append(msgs, sdkMsg)
	}

	grpcConn, err := utils.GetGRPC(c.grpcAddr)
	if err != nil {
		return "", errors.Wrap(err, "failed to get grpc connection")
	}

	wallet, ok := c.Wallets[walletID]
	if !ok {
		return "", errors.New("wallet not found")
	}
	cosmosAddress := wallet.GetAddress()

	// Get account for sequence and account number
	accountClient := accounttypes.NewQueryClient(grpcConn)
	accountRes, err := accountClient.AccountInfo(ctx, &accounttypes.QueryAccountInfoRequest{Address: cosmosAddress})
	if err != nil {
		return "", errors.Wrap(err, "failed to get account info")
	}

	txCfg := authtx.NewTxConfig(c.codec, authtx.DefaultSignModes)
	txBuilder := txCfg.NewTxBuilder()
	txBuilder.SetGasLimit(5000000)
	txBuilder.SetMsgs(msgs...)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("uatom", 5000000)))

	sigV2 := signing.SignatureV2{
		PubKey: wallet.PrivateKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(txCfg.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
		Sequence: accountRes.Info.Sequence,
	}
	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return "", errors.Wrap(err, "failed to set signature")
	}

	signerData := xauthsigning.SignerData{
		Address:       cosmosAddress,
		ChainID:       c.ChainID,
		AccountNumber: accountRes.Info.AccountNumber,
	}
	sigV2, err = tx.SignWithPrivKey(
		ctx,
		signing.SignMode(txCfg.SignModeHandler().DefaultMode()),
		signerData,
		txBuilder,
		wallet.PrivateKey,
		txCfg,
		accountRes.Info.Sequence,
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign with priv key")
	}
	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return "", errors.Wrap(err, "failed to set signature")
	}

	// Generated Protobuf-encoded bytes.
	txBytes, err := txCfg.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return "", errors.Wrap(err, "failed to encode tx")
	}

	txClient := txtypes.NewServiceClient(grpcConn)
	// We then call the BroadcastTx method on this client.
	grpcRes, err := txClient.BroadcastTx(
		ctx,
		&txtypes.BroadcastTxRequest{
			Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes, // Proto-binary of the signed transaction, see previous step.
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to broadcast tx")
	}
	if grpcRes.TxResponse.Code != 0 {
		return "", errors.Errorf("tx failed with code %d: %+v", grpcRes.TxResponse.Code, grpcRes.TxResponse)
	}

	return grpcRes.TxResponse.TxHash, nil
}
