package ethereum

import (
	"crypto/ecdsa"
	"encoding/hex"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/pkg/errors"
)

var _ network.Wallet = &Wallet{}

type Wallet struct {
	ID         string
	Address    ethcommon.Address
	PrivateKey *ecdsa.PrivateKey
}

// AddWallet implements network.Chain.
func (e *Ethereum) AddWallet(walletID string, privateKeyHex string) error {
	privKeyHexTrimmed := strings.TrimPrefix(privateKeyHex, "0x")
	keyBytes, err := hex.DecodeString(privKeyHexTrimmed)
	if err != nil {
		return errors.Wrap(err, "private key failed to decode")
	}
	privKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return errors.Wrap(err, "private key failed to convert to ECDSA")
	}

	e.Wallets[walletID] = Wallet{
		ID:         walletID,
		Address:    crypto.PubkeyToAddress(privKey.PublicKey),
		PrivateKey: privKey,
	}

	return nil
}

// GetWallet implements network.Chain.
func (e *Ethereum) GetWallet(walletID string) (network.Wallet, error) {
	wallet, ok := e.Wallets[walletID]
	if !ok {
		return nil, errors.Errorf("wallet not found: %s", walletID)
	}
	return &wallet, nil
}

// GenerateWallet implements network.Chain.
func (e *Ethereum) GenerateWallet(walletID string) (network.Wallet, error) {
	// Generate a new private key
	privKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate Ethereum private key")
	}

	// Create wallet
	wallet := Wallet{
		ID:         walletID,
		Address:    crypto.PubkeyToAddress(privKey.PublicKey),
		PrivateKey: privKey,
	}

	// Store wallet
	e.Wallets[walletID] = wallet

	return &wallet, nil
}

// GetWallets implements network.Chain.
func (e *Ethereum) GetWallets() []network.Wallet {
	wallets := make([]network.Wallet, 0, len(e.Wallets))
	for _, wallet := range e.Wallets {
		wallets = append(wallets, &wallet)
	}
	return wallets
}

// GetID implements network.Wallet.
func (w *Wallet) GetID() string {
	return w.ID
}

// GetAddress implements network.Wallet.
func (w *Wallet) GetAddress() string {
	return w.Address.String()
}

// GetPrivateKeyHex implements network.Wallet.
func (w *Wallet) GetPrivateKeyHex() string {
	return hex.EncodeToString(crypto.FromECDSA(w.PrivateKey))
}
