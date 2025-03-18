package cosmos

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/pkg/errors"
)

var _ network.Wallet = &Wallet{}

type Wallet struct {
	ID         string
	Address    cryptotypes.Address
	PrivateKey *secp256k1.PrivKey
}

func (c *Cosmos) AddWallet(walletID string, privateKeyHex string) error {
	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return fmt.Errorf("invalid key string: %w", err)
	}
	privKey := &secp256k1.PrivKey{Key: keyBytes}

	c.Wallets[walletID] = Wallet{
		ID:         walletID,
		Address:    privKey.PubKey().Address(),
		PrivateKey: privKey,
	}

	return nil
}

func (c *Cosmos) GetWallet(walletID string) (network.Wallet, error) {
	wallet, ok := c.Wallets[walletID]
	if !ok {
		return nil, errors.Errorf("wallet not found: %s", walletID)
	}
	return &wallet, nil
}

// GenerateWallet implements network.Chain.
func (c *Cosmos) GenerateWallet(walletID string) (network.Wallet, error) {
	// Generate a new private key
	privKey := secp256k1.GenPrivKey()

	// Create wallet
	wallet := Wallet{
		ID:         walletID,
		Address:    privKey.PubKey().Address(),
		PrivateKey: privKey,
	}

	// Store wallet
	c.Wallets[walletID] = wallet

	return &wallet, nil
}

// GetWallets implements network.Chain.
func (c *Cosmos) GetWallets() []network.Wallet {
	wallets := make([]network.Wallet, 0, len(c.Wallets))
	for _, wallet := range c.Wallets {
		wallets = append(wallets, &wallet)
	}
	return wallets
}

// GetAddress implements network.Wallet.
func (w *Wallet) GetAddress() string {
	return sdk.AccAddress(w.Address).String()
}

// GetID implements network.Wallet.
func (w *Wallet) GetID() string {
	return w.ID
}

// GetPrivateKeyHex implements network.Wallet.
func (w *Wallet) GetPrivateKeyHex() string {
	return hex.EncodeToString(w.PrivateKey.Key)
}
