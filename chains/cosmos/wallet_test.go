package cosmos

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWalletGetAddress(t *testing.T) {
	// Arrange
	const privateKeyHex = "8cb79e7fe3de7bfe364e0c5f3a89de39a1472bb67a33ee853d3215a19c476c27"
	const expectedAddress = "cosmos1maysgktd0ugpnrdkkyls8qyap83gk3wt7hxdp5"

	testLogger, _ := zap.NewDevelopment()
	cosmos, err := NewCosmos(testLogger, "test-chain-id", "cosmos", "uatom", "")
	require.NoError(t, err)

	// Act
	err = cosmos.AddWallet("test-wallet", privateKeyHex)

	// Assert
	require.NoError(t, err)
	require.Contains(t, cosmos.Wallets, "test-wallet")

	wallet := cosmos.Wallets["test-wallet"]
	address := wallet.Address()
	require.Equal(t, expectedAddress, address)
}
