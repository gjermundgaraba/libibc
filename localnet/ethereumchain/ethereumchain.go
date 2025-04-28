package ethereumchain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gjermundgaraba/libibc/chainclients/ethereum"
	"github.com/gjermundgaraba/libibc/chainclients/ethereum/beaconapi"
	"github.com/gjermundgaraba/libibc/chainclients/utils"
	"github.com/gjermundgaraba/libibc/localnet/ethereumchain/solidity"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/enclaves"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/services"
	"github.com/kurtosis-tech/kurtosis/api/golang/core/lib/starlark_run_config"
	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
	"go.uber.org/zap"
)

const (
	FaucetKeyName = "faucet"

	// ethereumPackageId is the package ID used by Kurtosis to find the Ethereum package we use for the testnet
	ethereumPackageId = "github.com/ethpandaops/ethereum-package@4.5.0"

	faucetPrivKeyHex = "0x04b9f63ecf84210c5366c66d68fa1f5da1fa4f634fad6dfc86178e4d79ff9e59"
)

type EthKurtosisChain struct {
	ChainClient *ethereum.Ethereum
	BeaconRPC   string

	kurtosisCtx      *kurtosis_context.KurtosisContext
	enclaveCtx       *enclaves.EnclaveContext
	executionService string
	consensusService string
}

func SpinUpEthereum(ctx context.Context, logger *zap.Logger, networkParams NetworkParams) (*EthKurtosisChain, error) {
	executionService := fmt.Sprintf("el-1-%s-%s", networkParams.Participants[0].ELType, networkParams.Participants[0].CLType)
	consensusService := fmt.Sprintf("cl-1-%s-%s", networkParams.Participants[0].CLType, networkParams.Participants[0].ELType)

	kurtosisCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to create kurtosis context: %w", err)
	}

	enclaveName := "ethereum-pos-testnet"
	enclaves, err := kurtosisCtx.GetEnclaves(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get enclaves: %w", err)
	}

	if enclaveInfos, found := enclaves.GetEnclavesByName()[enclaveName]; found {
		for _, enclaveInfo := range enclaveInfos {
			err = kurtosisCtx.DestroyEnclave(ctx, enclaveInfo.EnclaveUuid)
			if err != nil {
				return nil, fmt.Errorf("failed to destroy enclave: %w", err)
			}
		}
	}
	enclaveCtx, err := kurtosisCtx.CreateEnclave(ctx, enclaveName)
	if err != nil {
		return nil, fmt.Errorf("failed to create enclave: %w", err)
	}

	networkParamsJson, err := json.Marshal(networkParams)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal network params: %w", err)
	}
	starlarkResp, err := enclaveCtx.RunStarlarkRemotePackageBlocking(ctx, ethereumPackageId, &starlark_run_config.StarlarkRunConfig{
		SerializedParams: string(networkParamsJson),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run starlark script: %w", err)
	}
	fmt.Println(starlarkResp.RunOutput)

	// exeuctionCtx is the service context (kurtosis concept) for the execution node that allows us to get the public ports
	executionCtx, err := enclaveCtx.GetServiceContext(executionService)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution context: %w", err)
	}
	rpcPortSpec := executionCtx.GetPublicPorts()["rpc"]
	ethRPC := fmt.Sprintf("http://localhost:%d", rpcPortSpec.GetNumber())

	// consensusCtx is the service context (kurtosis concept) for the consensus node that allows us to get the public ports
	consensusCtx, err := enclaveCtx.GetServiceContext(consensusService)
	if err != nil {
		return nil, fmt.Errorf("failed to get consensus context: %w", err)
	}
	beaconPortSpec := consensusCtx.GetPublicPorts()["http"]
	beaconRPC := fmt.Sprintf("http://localhost:%d", beaconPortSpec.GetNumber())

	if networkParams.WaitForFinalization {
		var beaconAPIClient beaconapi.Client
		err = utils.WaitForCondition(30*time.Minute, 5*time.Second, func() (bool, error) {
			logger.Debug("Waiting for chain to finalize")

			beaconAPIClient, err = beaconapi.NewBeaconAPIClient(logger, beaconRPC)
			if err != nil {
				logger.Debug("Failed to create beacon API client", zap.Error(err))
				return false, nil
			}

			finalizedBlocksResp, err := beaconAPIClient.GetFinalizedBlocks(ctx)
			if err != nil {
				logger.Debug("Failed to get finalized blocks", zap.Error(err))
				return false, nil
			}
			if !finalizedBlocksResp.Finalized {
				logger.Debug("Finalized blocks not found")
				return false, nil
			}

			header, err := beaconAPIClient.GetBeaconBlockHeader(ctx, finalizedBlocksResp.Data.Message.Slot)
			if err != nil {
				logger.Debug("Failed to get beacon block header", zap.Error(err))
				return false, nil
			}

			bootstrap, err := beaconAPIClient.GetBootstrap(ctx, header.Data.Root)
			if err != nil {
				logger.Debug("Failed to get bootstrap", zap.Error(err))
				return false, nil
			}

			return bootstrap.Data.Header.Beacon.Slot != 0, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to wait for chain to finalize: %w", err)
		}
	}

	ethClient, err := ethclient.Dial(ethRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to dial ethereum client: %w", err)
	}
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}
	faucetPrivKey, err := crypto.ToECDSA(ethcommon.FromHex(faucetPrivKeyHex))
	if err != nil {
		return nil, fmt.Errorf("failed to get faucet private key: %w", err)
	}

	nonIBCChainClient, err := ethereum.NewNonIBCEthereum(
		ctx,
		logger,
		chainID.String(),
		ethRPC,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create non-IBC chain client: %w", err)
	}

	deployerWallet, err := nonIBCChainClient.GenerateWallet("deployer")
	if err != nil {
		return nil, fmt.Errorf("failed to generate deployer wallet: %w", err)
	}
	deployerPrivKeyHex := deployerWallet.PrivateKeyHex()
	deployerPrivKey, err := crypto.ToECDSA(ethcommon.FromHex(deployerPrivKeyHex))
	if err != nil {
		return nil, fmt.Errorf("failed to get deployer private key: %w", err)
	}

	nonIBCChainClient.AddWallet(FaucetKeyName, faucetPrivKeyHex)
	faucetPubKey := faucetPrivKey.PublicKey

	ibcContractAddresses, err := solidity.DeployIBC(
		ethRPC,
		deployerPrivKey,
		faucetPubKey,
	)

	nonIBCChainClient.ICS26Address = ethcommon.HexToAddress(ibcContractAddresses.Ics26Router)
	nonIBCChainClient.ICS20Address = ethcommon.HexToAddress(ibcContractAddresses.Ics20Transfer)
	nonIBCChainClient.RelayerHelperAddress = ethcommon.HexToAddress(ibcContractAddresses.RelayerHelper)

	chainClient := nonIBCChainClient

	return &EthKurtosisChain{
		ChainClient: chainClient,
		BeaconRPC:   beaconRPC,

		kurtosisCtx:      kurtosisCtx,
		enclaveCtx:       enclaveCtx,
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
