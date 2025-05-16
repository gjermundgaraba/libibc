module github.com/gjermundgaraba/libibc/eurekarelayer

go 1.23.8

replace github.com/gjermundgaraba/libibc/chainclients => ../chainclients

require (
	github.com/ethereum/go-ethereum v1.15.5
	github.com/gjermundgaraba/libibc/chainclients v0.0.0
	github.com/pkg/errors v0.9.1
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.71.0
	google.golang.org/protobuf v1.36.5
)

require (
	github.com/holiman/uint256 v1.3.2 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
)
