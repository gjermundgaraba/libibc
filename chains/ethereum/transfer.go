package ethereum

import (
	"context"
	"math/big"
	"time"

	"github.com/cosmos/solidity-ibc-eureka/packages/go-abigen/ics20transfer"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gjermundgaraba/libibc/chains/ethereum/erc20"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/ibc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SendTransfer implements network.Chain.
func (e *Ethereum) SendTransfer(
	ctx context.Context,
	clientID string,
	wallet network.Wallet,
	amount *big.Int,
	denom string,
	to string,
	memo string,
) (ibc.Packet, error) {
	ethereumWallet, ok := wallet.(*Wallet)
	if !ok {
		return ibc.Packet{}, errors.Errorf("invalid wallet type: %T", wallet)
	}

	ethClient, err := ethclient.Dial(e.ethRPC)
	if err != nil {
		return ibc.Packet{}, errors.Wrap(err, "failed to dial ethereum client")
	}

	erc20Address := ethcommon.HexToAddress(denom)
	erc20Contract, err := erc20.NewContract(erc20Address, ethClient)
	if err != nil {
		return ibc.Packet{}, errors.Wrap(err, "failed to get erc20 contract")
	}

	ics20Contract, err := ics20transfer.NewContract(e.ics20Address, ethClient)
	if err != nil {
		return ibc.Packet{}, errors.Wrap(err, "failed to get ics20 contract")
	}

	currentApproval, err := erc20Contract.Allowance(nil, ethereumWallet.address, e.ics20Address)
	if err != nil {
		return ibc.Packet{}, errors.Wrap(err, "failed to get current approval")
	}

	if currentApproval.Cmp(amount) < 0 {
		if _, err := e.Transact(ctx, ethereumWallet, func(ethClient *ethclient.Client, txOpts *bind.TransactOpts) (*ethtypes.Transaction, error) {
			return erc20Contract.Approve(txOpts, e.ics20Address, amount)
		}); err != nil {
			return ibc.Packet{}, errors.Wrap(err, "failed to approve transfer")
		}

		e.logger.Info("Approved transfer", zap.Uint64("amount", amount.Uint64()), zap.String("denom", denom), zap.String("to", to))
		time.Sleep(5 * time.Second)
	}

	timeout := uint64(time.Now().Add(6 * time.Hour).Unix())
	sendTransferMsg := ics20transfer.IICS20TransferMsgsSendTransferMsg{
		Denom:            erc20Address,
		Amount:           amount,
		Receiver:         to,
		SourceClient:     clientID,
		DestPort:         "transfer",
		TimeoutTimestamp: timeout,
		Memo:             memo,
	}

	receipt, err := e.Transact(ctx, ethereumWallet, func(ethClient *ethclient.Client, txOpts *bind.TransactOpts) (*ethtypes.Transaction, error) {
		return ics20Contract.SendTransfer(txOpts, sendTransferMsg)
	})
	if err != nil {
		return ibc.Packet{}, errors.Wrap(err, "failed to send transfer")
	}

	packets, err := e.GetPackets(ctx, receipt.TxHash.String())
	if err != nil {
		return ibc.Packet{}, errors.Wrapf(err, "failed to get packets for transfer with tx hash: %s", receipt.TxHash.String())
	}
	if len(packets) != 1 {
		return ibc.Packet{}, errors.Errorf("failed to get packet for transfer (expected 1, got %d)", len(packets))
	}

	e.logger.Info("Sent transfer", zap.String("tx_hash", receipt.TxHash.String()), zap.String("from", wallet.Address()), zap.String("to", to), zap.Uint64("amount", amount.Uint64()), zap.String("denom", denom))

	return packets[0], nil
}
