package cosmos

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	// TestCosmosGRPC is the gRPC address of the Cosmos node
	TestCosmosGRPC  = "eureka-devnet-node-01-grpc.dev.skip.build:443"
	TestTxHashIBCV1 = "C2B9030069B1172A9685EC710D661D61462D69AC06E90582330013C76AB1F23C"
	TestTxHashIBCV2 = "096ED04AB0A2B0169F16703900A8AA7F3915DBAFA359166EE8DD07B397290F8E"
)

func TestGetPacketV1(t *testing.T) {
	ctx := context.Background()

	// Create a test logger
	testLogger, _ := zap.NewDevelopment()

	// Create a new Cosmos instance
	cosmos, err := NewCosmos(testLogger, "test-chain-id", TestCosmosGRPC)
	if err != nil {
		t.Fatal(err)
	}

	// Query a transaction
	resp, err := cosmos.QueryTx(ctx, TestTxHashIBCV1)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Get the packet from the transaction
	packets, err := cosmos.GetPackets(ctx, TestTxHashIBCV1)
	require.NoError(t, err)
	require.Len(t, packets, 1)
	fmt.Printf("Packets: %+v\n", packets)
}

func TestGetPacketV2(t *testing.T) {
	ctx := context.Background()

	// Create a test logger
	testLogger, _ := zap.NewDevelopment()

	// Create a new Cosmos instance
	cosmos, err := NewCosmos(testLogger, "test-chain-id", TestCosmosGRPC)
	if err != nil {
		t.Fatal(err)
	}

	// Query a transaction
	resp, err := cosmos.QueryTx(ctx, TestTxHashIBCV2)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Get the packet from the transaction
	packets, err := cosmos.GetPackets(ctx, TestTxHashIBCV2)
	require.NoError(t, err)
	require.Len(t, packets, 1)
	fmt.Printf("Packets: %+v\n", packets)
}
