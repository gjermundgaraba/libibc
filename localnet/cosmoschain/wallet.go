package cosmoschain

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Wallet struct {
	Mnemonic string
	Address  []byte
	KeyName  string
	chainCfg ChainConfig
}

type WalletAmount struct {
	Address string
	Denom   string
	Amount  sdkmath.Int
}

func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ChainConfig) Wallet {
	return Wallet{
		Mnemonic: mnemonic,
		Address:  address,
		KeyName:  keyname,
		chainCfg: chainCfg,
	}
}

// Get formatted address, passing in a prefix
func (w Wallet) FormattedAddress() string {
	return sdk.MustBech32ifyAddressBytes(w.chainCfg.Bech32Prefix, w.Address)
}
