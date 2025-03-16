package cosmos

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/gjermundgaraba/libibc/chains/network"
)

var _ network.Wallet = &Wallet{}

type Wallet struct {
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
		Address:    privKey.PubKey().Address(),
		PrivateKey: privKey,
	}

	return nil
}

// GetAddress implements network.Wallet.
func (w *Wallet) GetAddress() string {
	return w.Address.String()
}
