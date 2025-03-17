package utils

import (
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func GetGRPC(addr string) (*grpc.ClientConn, error) {
	if strings.Contains(addr, "443") {
		return getTLSGRPC(addr)
	} else {
		return getNonTLSGRPC(addr)
	}
}

func getTLSGRPC(addr string) (*grpc.ClientConn, error) {
	creds := credentials.NewTLS(nil)

	// Establish a secure connection with the gRPC server
	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(creds),
		retryConfig(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to grpc client with addr: %s: %w", addr, err)
	}

	return conn, nil
}

func getNonTLSGRPC(addr string) (*grpc.ClientConn, error) {
	// Establish a connection with the gRPC server
	conn, err := grpc.Dial(
		addr,
		grpc.WithInsecure(),
		retryConfig(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to grpc client with addr: %s: %w", addr, err)
	}

	return conn, nil
}

func retryConfig() grpc.DialOption {
	policy := `{
            "methodConfig": [{
                "name": [{}],
                "retryPolicy": {
                    "MaxAttempts": 4,
                    "InitialBackoff": ".01s",
                    "MaxBackoff": ".01s",
                    "BackoffMultiplier": 1.0,
                    "RetryableStatusCodes": [
						"CANCELLED",
						"UNKNOWN",
						"DEADLINE_EXCEEDED",
						"NOT_FOUND",
						"ALREADY_EXISTS",
						"PERMISSION_DENIED",
						"RESOURCE_EXHAUSTED",
						"FAILED_PRECONDITION",
						"ABORTED",
						"OUT_OF_RANGE",
						"UNIMPLEMENTED",
						"INTERNAL",
						"UNAVAILABLE",
						"DATA_LOSS",
						"UNAUTHENTICATED"
				    ]
                }
            }]
        }`

	return grpc.WithDefaultServiceConfig(policy)
}
