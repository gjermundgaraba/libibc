package cosmos

import (
	"context"
	"time"

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
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SubmitTx implements network.Chain.
func (c *Cosmos) SubmitTx(ctx context.Context, txBz []byte, wallet network.Wallet) (string, error) {
	cosmosWallet, ok := wallet.(*Wallet)
	if !ok {
		return "", errors.Errorf("invalid wallet type: %T", wallet)
	}

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

	grpcRes, err := c.submitTx(ctx, cosmosWallet, 5_000_000, msgs...)
	if err != nil {
		return "", errors.Wrap(err, "failed to submit tx")
	}

	return grpcRes.TxResponse.TxHash, nil
}

func (c *Cosmos) submitTx(ctx context.Context, wallet *Wallet, gas uint64, msgs ...sdk.Msg) (*txtypes.BroadcastTxResponse, error) {
	grpcConn, err := utils.GetGRPC(c.grpcAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get grpc connection")
	}

	// Get account for sequence and account number
	accountClient := accounttypes.NewQueryClient(grpcConn)
	accountRes, err := accountClient.AccountInfo(ctx, &accounttypes.QueryAccountInfoRequest{Address: wallet.Address()})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get account info")
	}

	txCfg := authtx.NewTxConfig(c.codec, authtx.DefaultSignModes)
	txBuilder := txCfg.NewTxBuilder()
	txBuilder.SetGasLimit(gas)
	txBuilder.SetMsgs(msgs...)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin(c.GasDenom, int64(gas))))

	sigV2 := signing.SignatureV2{
		PubKey: wallet.privateKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(txCfg.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
		Sequence: accountRes.Info.Sequence,
	}
	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set signature")
	}

	signerData := xauthsigning.SignerData{
		Address:       wallet.Address(),
		ChainID:       c.ChainID,
		AccountNumber: accountRes.Info.AccountNumber,
	}
	sigV2, err = tx.SignWithPrivKey(
		ctx,
		signing.SignMode(txCfg.SignModeHandler().DefaultMode()),
		signerData,
		txBuilder,
		wallet.privateKey,
		txCfg,
		accountRes.Info.Sequence,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign with priv key")
	}
	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set signature")
	}

	// Generated Protobuf-encoded bytes.
	txBytes, err := txCfg.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode tx")
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
		return nil, errors.Wrap(err, "failed to broadcast tx")
	}
	if grpcRes.TxResponse.Code != 0 {
		return nil, errors.Errorf("tx failed with code %d: %+v", grpcRes.TxResponse.Code, grpcRes.TxResponse)
	}

	// Just to give it some time to index the tx properly
	time.Sleep(10 * time.Second)

	txResp, err := txClient.GetTx(ctx, &txtypes.GetTxRequest{Hash: grpcRes.TxResponse.TxHash})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query tx %s", grpcRes.TxResponse.TxHash)
	}
	if txResp.TxResponse.Code != 0 {
		return nil, errors.Errorf("tx %s failed with code %d: %+v", grpcRes.TxResponse.TxHash, txResp.TxResponse.Code, txResp.TxResponse)
	}

	c.logger.Info("tx broadcasted", zap.String("tx_hash", grpcRes.TxResponse.TxHash))

	return grpcRes, nil
}
