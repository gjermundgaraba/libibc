package ethereum

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SubmitTx implements network.Chain.
func (e *Ethereum) SubmitRelayTx(ctx context.Context, txBz []byte, wallet network.Wallet) (string, error) {
	ethereumWallet, ok := wallet.(*Wallet)
	if !ok {
		return "", errors.Errorf("invalid wallet type: %T", wallet)
	}

	receipt, err := e.Transact(ctx, ethereumWallet, func(ethClient *ethclient.Client, txOpts *bind.TransactOpts) (*ethtypes.Transaction, error) {
		unsignedTx := ethtypes.NewTransaction(
			txOpts.Nonce.Uint64(),
			e.ics26Address,
			new(big.Int).SetUint64(0),
			15_000_000,
			txOpts.GasPrice,
			txBz,
		)

		signedTx, err := txOpts.Signer(txOpts.From, unsignedTx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to sign tx")
		}

		err = ethClient.SendTransaction(ctx, signedTx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to submit tx")
		}

		return signedTx, nil
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to submit tx")
	}

	e.logger.Info("Submitted relay tx", zap.String("tx_hash", receipt.TxHash.String()))

	return receipt.TxHash.String(), nil
}

func (e *Ethereum) Transact(ctx context.Context, wallet *Wallet, doTx func(*ethclient.Client, *bind.TransactOpts) (*ethtypes.Transaction, error)) (*ethtypes.Receipt, error) {
	ethClient, err := ethclient.Dial(e.ethRPC)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial ethereum client")
	}

	txOpts, err := GetTransactOpts(ctx, ethClient, e.actualChainID, wallet.privateKey, 5)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transact opts")
	}

	tx, err := doTx(ethClient, txOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do transaction")
	}

	receipt, err := WaitForReceipt(ctx, ethClient, tx.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get receipt")
	}

	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return nil, errors.New("failed to approve transfer")
	}

	return receipt, nil
}

func GetTransactOpts(ctx context.Context, ethClient *ethclient.Client, chainID *big.Int, key *ecdsa.PrivateKey, extraGwei int64) (*bind.TransactOpts, error) {
	fromAddress := crypto.PubkeyToAddress(key.PublicKey)

	txOpts, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create transactor")
	}

	// Get the suggested gas price from the client.
	suggestedGasPrice, err := ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get suggested gas price")
	}

	txOpts.GasPrice = new(big.Int).Add(suggestedGasPrice, big.NewInt(extraGwei*1000000000)) // Add extra Gwei

	nonce, err := ethClient.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		nonce = 0
	}
	txOpts.Nonce = big.NewInt(int64(nonce))

	// header, err := ethClient.HeaderByNumber(ctx, nil)
	// if err != nil {
	// 	panic(err)
	// }
	//
	// // For EIP-1559 transactions: double the gas tip and fee cap.
	// tipCap, err := ethClient.SuggestGasTipCap(ctx)
	// if err != nil {
	// 	panic(err)
	// }
	// txOpts.GasTipCap = new(big.Int).Mul(tipCap, big.NewInt(5))
	// // Compute the gas fee cap by doubling the sum of header.BaseFee and the original tipCap.
	// txOpts.GasFeeCap = new(big.Int).Mul(new(big.Int).Add(header.BaseFee, tipCap), big.NewInt(5))

	return txOpts, nil
}

func WaitForReceipt(ctx context.Context, ethClient *ethclient.Client, hash ethcommon.Hash) (*ethtypes.Receipt, error) {

	var receipt *ethtypes.Receipt
	if err := utils.WaitForCondition(time.Second*120, time.Second, func() (bool, error) {
		var err error
		receipt, err = ethClient.TransactionReceipt(ctx, hash)
		if err != nil {
			return false, nil
		}

		return receipt != nil, nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for transaction receipt")
	}

	return receipt, nil
}
