package ethereum

import (
	"context"
	"math/big"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gjermundgaraba/libibc/chains/ethereum/erc20"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (e *Ethereum) Send(ctx context.Context, walletID string, amount *big.Int, denom string, toAddress string) (string, error) {
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
	
	var tx *ethtypes.Transaction
	var receipt *ethtypes.Receipt
	
	// Check if we're sending native ETH or an ERC20 token
	if strings.ToLower(denom) == "eth" {
		// Native ETH transfer
		tx = ethtypes.NewTransaction(
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
		receipt, err = WaitForReceipt(ctx, ethClient, signedTx.Hash())
		if err != nil {
			return "", errors.Wrap(err, "failed to get transaction receipt")
		}
		
		e.logger.Info("ETH send transaction successful", 
			zap.String("tx_hash", receipt.TxHash.String()), 
			zap.String("from", wallet.Address.String()), 
			zap.String("to", toAddress), 
			zap.String("amount", amount.String()),
			zap.String("denom", denom))
	} else {
		// ERC20 token transfer
		erc20Address := ethcommon.HexToAddress(denom)
		contract, err := erc20.NewContract(erc20Address, ethClient)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create ERC20 contract instance for %s", denom)
		}
		
		// Set gas limit higher for ERC20 transfers
		txOpts.GasLimit = 100000
		
		tx, err := contract.Transfer(txOpts, to, amount)
		if err != nil {
			return "", errors.Wrapf(err, "failed to send ERC20 transaction for token %s", denom)
		}
		
		receipt, err = WaitForReceipt(ctx, ethClient, tx.Hash())
		if err != nil {
			return "", errors.Wrap(err, "failed to get transaction receipt")
		}
		
		e.logger.Info("ERC20 send transaction successful", 
			zap.String("tx_hash", receipt.TxHash.String()), 
			zap.String("from", wallet.Address.String()), 
			zap.String("to", toAddress), 
			zap.String("amount", amount.String()),
			zap.String("token", denom))
	}
	
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return "", errors.New("transaction failed")
	}
	
	return receipt.TxHash.String(), nil
}