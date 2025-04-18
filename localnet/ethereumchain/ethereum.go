package ethereumchain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gjermundgaraba/libibc/chains/ethereum"
	"github.com/gjermundgaraba/libibc/chains/ethereum/beaconapi"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/gjermundgaraba/libibc/utils"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
	"go.uber.org/zap"
)

const (
	// ethereumPackageId is the package ID used by Kurtosis to find the Ethereum package we use for the testnet
	ethereumPackageId = "github.com/ethpandaops/ethereum-package@4.5.0"

	faucetPrivateKey = "0x04b9f63ecf84210c5366c66d68fa1f5da1fa4f634fad6dfc86178e4d79ff9e59"
)

type EthKurtosisChain struct {
	*ethereum.Ethereum

	kurtosisCtx      *kurtosis_context.KurtosisContext
	enclaveCtx       *enclaves.EnclaveContext
	executionService string
	consensusService string
}

func SpinUpEthereum(ctx context.Context, logger *zap.Logger, networkParams NetworkParams) (network.Chain, error) {
	executionService := fmt.Sprintf("el-1-%s-%s", networkParams.Participants[0].ELType, networkParams.Participants[0].CLType)
	consensusService := fmt.Sprintf("cl-1-%s-%s", networkParams.Participants[0].CLType, networkParams.Participants[0].ELType)

	kurtosisCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	if err != nil {
		return nil, err
	}

	enclaveName := "ethereum-pos-testnet"
	enclaves, err := kurtosisCtx.GetEnclaves(ctx)
	if err != nil {
		return nil, err
	}

	if enclaveInfos, found := enclaves.GetEnclavesByName()[enclaveName]; found {
		for _, enclaveInfo := range enclaveInfos {
			err = kurtosisCtx.DestroyEnclave(ctx, enclaveInfo.EnclaveUuid)
			if err != nil {
				return nil, err
			}
		}
	}
	enclaveCtx, err := kurtosisCtx.CreateEnclave(ctx, enclaveName)
	if err != nil {
		return nil, err
	}

	networkParamsJson, err := json.Marshal(networkParams)
	if err != nil {
		return nil, err
	}
	starlarkResp, err := enclaveCtx.RunStarlarkRemotePackageBlocking(ctx, ethereumPackageId, &starlark_run_config.StarlarkRunConfig{
		SerializedParams: string(networkParamsJson),
	})
	if err != nil {
		return nil, err
	}
	fmt.Println(starlarkResp.RunOutput)

	// exeuctionCtx is the service context (kurtosis concept) for the execution node that allows us to get the public ports
	executionCtx, err := enclaveCtx.GetServiceContext(executionService)
	if err != nil {
		return nil, err
	}
	rpcPortSpec := executionCtx.GetPublicPorts()["rpc"]
	ethRPC := fmt.Sprintf("http://localhost:%d", rpcPortSpec.GetNumber())

	// consensusCtx is the service context (kurtosis concept) for the consensus node that allows us to get the public ports
	consensusCtx, err := enclaveCtx.GetServiceContext(consensusService)
	if err != nil {
		return nil, err
	}
	beaconPortSpec := consensusCtx.GetPublicPorts()["http"]
	beaconRPC := fmt.Sprintf("http://localhost:%d", beaconPortSpec.GetNumber())

	if networkParams.WaitForFinalization {
		var beaconAPIClient beaconapi.Client
		err = utils.WaitForCondition(30*time.Minute, 5*time.Second, func() (bool, error) {
			beaconAPIClient, err = beaconapi.NewBeaconAPIClient(beaconRPC)
			if err != nil {
				return false, nil
			}

			finalizedBlocksResp, err := beaconAPIClient.GetFinalizedBlocks()
			fmt.Printf("Waiting for chain to finalize, finalizedBlockResp: %+v, err: %s\n", finalizedBlocksResp, err)
			if err != nil {
				return false, nil
			}
			if !finalizedBlocksResp.Finalized {
				return false, nil
			}

			header, err := beaconAPIClient.GetBeaconBlockHeader(finalizedBlocksResp.Data.Message.Slot)
			if err != nil {
				return false, nil
			}
			bootstrap, err := beaconAPIClient.GetBootstrap(header.Data.Root)
			if err != nil {
				return false, nil
			}

			return bootstrap.Data.Header.Beacon.Slot != 0, nil
		})
		if err != nil {
			return nil, err
		}
	}

	ethClient, err := ethclient.Dial(ethRPC)
	if err != nil {
		return nil, err
	}
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	faucetPrivKey, err := crypto.ToECDSA(ethcommon.FromHex(faucetPrivateKey))
	if err != nil {
		return nil, err
	}

	chain, err := ethereum.NewEthereumWithDeploy(
		ctx,
		logger,
		chainID.String(),
		ethRPC,
		faucetPrivKey,
	)
	if err != nil {
		return nil, err
	}

	return &EthKurtosisChain{
		Ethereum: chain,

		kurtosisCtx:      kurtosisCtx,
		executionService: executionService,
		consensusService: consensusService,
	}, nil
}

func (e EthKurtosisChain) Destroy(ctx context.Context) {
	if err := e.kurtosisCtx.DestroyEnclave(ctx, string(e.enclaveCtx.GetEnclaveUuid())); err != nil {
		panic(err)
	}
}

func (e EthKurtosisChain) DumpLogs(ctx context.Context) error {
	enclaveServices, err := e.enclaveCtx.GetServices()
	if err != nil {
		return err
	}

	userServices := make(map[services.ServiceUUID]bool)
	serviceIdToName := make(map[services.ServiceUUID]string)
	for serviceName, servicesUUID := range enclaveServices {
		userServices[servicesUUID] = true
		serviceIdToName[servicesUUID] = string(serviceName)

	}

	stream, cancelFunc, err := e.kurtosisCtx.GetServiceLogs(ctx, string(e.enclaveCtx.GetEnclaveUuid()), userServices, false, true, 0, nil)
	if err != nil {
		return err
	}

	// Dump the stream chan into stdout
	fmt.Println("Dumping kurtosis logs")
	for {
		select {
		case logs, ok := <-stream:
			if !ok {
				return nil
			}
			for serviceID, serviceLog := range logs.GetServiceLogsByServiceUuids() {
				if serviceIdToName[serviceID] != e.executionService {
					continue
				}
				for _, log := range serviceLog {
					fmt.Printf("Service %s logs: %s\n", serviceIdToName[serviceID], log)
				}
			}
		case <-ctx.Done():
			cancelFunc()
			return nil
		}
	}
}
