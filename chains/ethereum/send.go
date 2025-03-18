package ethereum

import (
	"context"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (e *Ethereum) NativeSend(ctx context.Context, walletID string, amount *big.Int, toAddress string) (string, error) {
	wallet, ok := e.Wallets[walletID]
	if !ok {
		return "", errors.New("wallet not found")
	}
	
	ethClient, err := ethclient.Dial(e.ethRPC)
	if err != nil {
		return "", errors.Wrap(err, "failed to dial ethereum client")
	}
	
	to := ethcommon.HexToAddress(toAddress)
	txOpts, err := GetTransactOpts(ctx, ethClient, e.actualChainID, wallet.PrivateKey, 5)
	if err != nil {
		return "", errors.Wrap(err, "failed to get transaction options")
	}
	
	tx := ethtypes.NewTransaction(
		txOpts.Nonce.Uint64(),
		to,
		amount,
		21000, // Standard gas limit for ETH transfers
		txOpts.GasPrice,
		nil, // No data for simple ETH transfers
	)
	signedTx, err := txOpts.Signer(txOpts.From, tx)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign transaction")
	}
	
	err = ethClient.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", errors.Wrap(err, "failed to send transaction")
	}
	receipt, err := WaitForReceipt(ctx, ethClient, signedTx.Hash())
	if err != nil {
		return "", errors.Wrap(err, "failed to get transaction receipt")
	}
	
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return "", errors.New("transaction failed")
	}
	
	e.logger.Info("Native send transaction successful", zap.String("tx_hash", receipt.TxHash.String()), zap.String("from", wallet.Address.String()), zap.String("to", toAddress), zap.String("amount", amount.String()))
	
	return receipt.TxHash.String(), nil
}