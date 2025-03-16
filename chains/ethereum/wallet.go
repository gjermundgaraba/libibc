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
		Address:    crypto.PubkeyToAddress(privKey.PublicKey),
		PrivateKey: privKey,
	}

	return nil
}

// GetAddress implements network.Wallet.
func (w *Wallet) GetAddress() string {
	return w.Address.String()
}
