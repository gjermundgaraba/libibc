package cosmos

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gjermundgaraba/libibc/chains/network"
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
