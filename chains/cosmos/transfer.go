package cosmos

import (
	"context"
	"math/big"

	"github.com/gjermundgaraba/libibc/ibc"
)

// SendTransfer implements network.Chain.
func (c *Cosmos) SendTransfer(
	ctx context.Context,
	clientID string,
	walletID string,
	amount *big.Int,
	denom string,
	to string,
) (ibc.Packet, error) {
	return ibc.Packet{}, nil
}
