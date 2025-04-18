package cosmoschain

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/gjermundgaraba/libibc/localnet/cosmoschain/dockerutils"
)

// ChainConfig defines the chain parameters requires to run an interchaintest testnet for a chain.
type ChainConfig struct {
	// Chain name, e.g. cosmoshub.
	Name string
	// Chain ID, e.g. cosmoshub-4
	ChainID string
	// Docker image required for running chain nodes.
	Image dockerutils.ImageRef
	// Binary to execute for the chain node daemon.
	Bin string
	// Bech32 prefix for chain addresses, e.g. cosmos.
	Bech32Prefix string
	// Denomination of native currency, e.g. uatom.
	Denom string
	// Coin type
	CoinType string
	// Key signature algorithm
	SigningAlgorithm string
	// Minimum gas prices for sending transactions, in native currency denom.
	GasPrices string
	// Adjustment multiplier for gas fees.
	GasAdjustment float64
	// Trusting period of the chain.
	TrustingPeriod string
	// // Do not use docker host mount.
	// NoHostMount bool `yaml:"no-host-mount"`
	// // When true, will skip validator gentx flow
	// SkipGenTx bool
	// When provided, genesis file contents will be altered before sharing for genesis.
	ModifyGenesis func(ChainConfig, []byte) ([]byte, error)
	// Modify genesis-amounts for the validator at the given index
	ModifyGenesisAmounts func(int) (sdk.Coin, sdk.Coin)
	// Override config parameters for files at filepath.
	ConfigFileOverrides map[string]any
	// Non-nil will override the encoding config, used for cosmos chains only.
	EncodingConfig *testutil.TestEncodingConfig
	// Required when the chain requires the chain-id field to be populated for certain commands
	UsingChainIDFlagCLI bool
	// CoinDecimals for the chains base micro/nano/atto token configuration.
	CoinDecimals int64
	// HostPortOverride exposes ports to the host
	HostPortOverride map[int]int
	// Additional start command arguments
	AdditionalStartArgs []string
	// Environment variables for chain nodes
	Env []string
}
