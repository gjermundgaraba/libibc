package ethereum

import (
	"context"
	"math/big"

	"github.com/cosmos/solidity-ibc-eureka/abigen/ics26router"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gjermundgaraba/libibc/chains/ethereum/erc20"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var _ network.Chain = &Ethereum{}

type Ethereum struct {
	ChainID string
	Clients map[string]network.ClientCounterparty
	Wallets map[string]Wallet

	actualChainID *big.Int
	ethRPC        string
	ics26Address  ethcommon.Address
	ics20Address  ethcommon.Address
	logger        *zap.Logger
}

func NewEthereum(ctx context.Context, logger *zap.Logger, chainID string, ethRPC string, ics26AddressHex string) (*Ethereum, error) {
	ethClient, err := ethclient.Dial(ethRPC)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial ethereum client")
	}

	ethChainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ethereum chain ID")
	}

	ics26Address := ethcommon.HexToAddress(ics26AddressHex)
	router, err := ics26router.NewContract(ics26Address, ethClient)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ics26 router contract")
	}

	ics20Address, err := router.GetIBCApp(nil, "transfer")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ics20 address")
	}

	return &Ethereum{
		ChainID: chainID,
		Clients: make(map[string]network.ClientCounterparty),
		Wallets: make(map[string]Wallet),

		actualChainID: ethChainID,
		ethRPC:        ethRPC,
		ics26Address:  ics26Address,
		ics20Address:  ics20Address,
		logger:        logger,
	}, nil
}

// GetChainID implements network.Chain.
func (e *Ethereum) GetChainID() string {
	return e.ChainID
}

// AddClient implements network.Chain.
func (e *Ethereum) AddClient(clientID string, counterparty network.ClientCounterparty) {
	e.Clients[clientID] = counterparty
}

// GetClients implements network.Chain.
func (e *Ethereum) GetClients() map[string]network.ClientCounterparty {
	return e.Clients
}

// GetBalance implements network.Chain.
func (e *Ethereum) GetBalance(ctx context.Context, address string, denom string) (*big.Int, error) {
	client, err := ethclient.Dial(e.ethRPC)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial ethereum client")
	}

	ethAddress := ethcommon.HexToAddress(address)

	// Handle ETH balance
	if denom == "eth" || denom == "ETH" {
		balance, err := client.BalanceAt(ctx, ethAddress, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to query ETH balance for address %s", address)
		}
		return balance, nil
	}

	// Handle ERC20 token balance
	erc20Address := ethcommon.HexToAddress(denom)
	erc20, err := erc20.NewContract(erc20Address, client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create ERC20 contract instance for %s", denom)
	}

	balance, err := erc20.BalanceOf(nil, ethAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query ERC20 balance for token %s and address %s", denom, address)
	}

	return balance, nil
}
