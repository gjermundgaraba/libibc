package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.toml")

	configContent := `
[[chains]]
chain-type = "cosmos"
chain-id = "test-chain"
grpc-addr = "localhost:9090"

[[chains.clients]]
client-id = "07-tendermint-0"
counterparty-chain-id = "counterparty-chain"
counterparty-id = "07-tendermint-1"

[[chains]]
chain-type = "cosmos"
chain-id = "test-chain-2"
grpc-addr = "localhost:9091"

[[chains.clients]]
client-id = "07-tendermint-2"
counterparty-chain-id = "counterparty-chain-2"
counterparty-id = "07-tendermint-3"

[[chains.clients]]
client-id = "07-tendermint-4"
counterparty-chain-id = "counterparty-chain-3"
counterparty-id = "07-tendermint-5"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	// Load the config
	config, err := LoadConfig(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// Verify config values for first chain
	assert.Len(t, config.Chains, 2)
	assert.Equal(t, "cosmos", config.Chains[0].ChainType)
	assert.Equal(t, "test-chain", config.Chains[0].ChainID)
	assert.Equal(t, "localhost:9090", config.Chains[0].GRPCAddr)
	assert.Len(t, config.Chains[0].Clients, 1)
	assert.Equal(t, "07-tendermint-0", config.Chains[0].Clients[0].ClientID)
	assert.Equal(t, "counterparty-chain", config.Chains[0].Clients[0].CounterpartyChainID)
	assert.Equal(t, "07-tendermint-1", config.Chains[0].Clients[0].CounterpartyID)
	
	// Verify config values for second chain
	assert.Equal(t, "cosmos", config.Chains[1].ChainType)
	assert.Equal(t, "test-chain-2", config.Chains[1].ChainID)
	assert.Equal(t, "localhost:9091", config.Chains[1].GRPCAddr)
	assert.Len(t, config.Chains[1].Clients, 2)
	
	// Check first client of second chain
	assert.Equal(t, "07-tendermint-2", config.Chains[1].Clients[0].ClientID)
	assert.Equal(t, "counterparty-chain-2", config.Chains[1].Clients[0].CounterpartyChainID)
	assert.Equal(t, "07-tendermint-3", config.Chains[1].Clients[0].CounterpartyID)
	
	// Check second client of second chain
	assert.Equal(t, "07-tendermint-4", config.Chains[1].Clients[1].ClientID)
	assert.Equal(t, "counterparty-chain-3", config.Chains[1].Clients[1].CounterpartyChainID)
	assert.Equal(t, "07-tendermint-5", config.Chains[1].Clients[1].CounterpartyID)
}

func TestLoadConfig_Error(t *testing.T) {
	// Test with non-existent file
	config, err := LoadConfig("non_existent_file.toml")
	assert.Error(t, err)
	assert.Nil(t, config)
}