package cmd

import (
	"encoding/hex"
	"fmt"
	"strings"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/gogoproto/proto"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/v10/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
	"github.com/cosmos/solidity-ibc-eureka/packages/go-abigen/sp1ics07tendermint"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/gjermundgaraba/libibc/apis/eurekarelayerapi"
	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type createClientType string

const (
	sp1ICS07TendermintContractBytecodeLength = 14308

	createCosmosClientType createClientType = "cosmos"
	createEthClientType    createClientType = "eth"
)

func createClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-client [cosmos|eth] [from-chain-id] [to-chain-id] [key]... [value]...",
		Short: "Create client",
		Args:  cobra.MinimumNArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			logWriter.AddExtraLogger(func(entry string) {
				cmd.Println(entry)
			})

			clientTypeStr := args[0]
			var clientType createClientType
			switch clientTypeStr {
			case string(createCosmosClientType):
				clientType = createCosmosClientType
			case string(createEthClientType):
				clientType = createEthClientType
			default:
				return errors.Errorf("unknown client type %s", clientTypeStr)
			}

			srcChainID := args[1]
			dstChainID := args[2]
			params := make(map[string]string)
			for i := 3; i < len(args); i += 2 {
				if i+1 >= len(args) {
					return errors.New("missing value for key")
				}

				key := args[i]
				value := args[i+1]
				params[key] = value
			}

			eurekaClient := eurekarelayerapi.NewClient(logger, "localhost:3000")

			resp, err := eurekaClient.CreateClient(ctx, srcChainID, dstChainID, params)
			if err != nil {
				return errors.Wrap(err, "failed to create client")
			}

			createClientTxBz := resp.Tx

			switch clientType {
			case createCosmosClientType:
				if err := printCosmosClientCreation(createClientTxBz); err != nil {
					return errors.Wrap(err, "failed to print client creation")
				}
			case createEthClientType:
				if err := printEthClientCreation(createClientTxBz); err != nil {
					return errors.Wrap(err, "failed to print client creation")
				}
			default:
				return errors.Errorf("unknown client type %s", clientType)
			}

			return nil
		},
	}

	return cmd
}

func printCosmosClientCreation(createClientTxBz []byte) error {
	cosmosCodec := cosmos.SetupCodec()

	// Extract messages from the response (cosmos specific)
	var txBody txtypes.TxBody
	if err := proto.Unmarshal(createClientTxBz, &txBody); err != nil {
		return errors.Wrapf(err, "failed to unmarshal tx body")
	}

	if len(txBody.Messages) != 1 {
		return errors.New("expected exactly one message in tx")
	}

	anyMsg := txBody.Messages[0]
	var sdkMsg sdk.Msg
	if err := cosmosCodec.InterfaceRegistry().UnpackAny(anyMsg, &sdkMsg); err != nil {
		return errors.Wrapf(err, "failed to unpack message")
	}

	createClientMsg, ok := sdkMsg.(*clienttypes.MsgCreateClient)
	if !ok {
		return errors.New("expected MsgCreateClient message")
	}

	var expWasmClientState exported.ClientState
	if err := cosmosCodec.UnpackAny(createClientMsg.ClientState, &expWasmClientState); err != nil {
		return errors.Wrapf(err, "failed to unpack client state")
	}
	wasmClientState, ok := expWasmClientState.(*ibcwasmtypes.ClientState)
	if !ok {
		return errors.New("expected Wasm client state")
	}

	var expWasmConsensusState exported.ConsensusState
	if err := cosmosCodec.UnpackAny(createClientMsg.ConsensusState, &expWasmConsensusState); err != nil {
		return errors.Wrapf(err, "failed to unpack consensus state")
	}
	wasmConsensusState, ok := expWasmConsensusState.(*ibcwasmtypes.ConsensusState)
	if !ok {
		return errors.New("expected Wasm consensus state")
	}

	wasmClientStateAny, err := codectypes.NewAnyWithValue(wasmClientState)
	if err != nil {
		return errors.Wrapf(err, "failed to pack message")
	}
	wasmClientStateJson, err := cosmosCodec.MarshalJSON(wasmClientStateAny)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal migrate client message")
	}

	wasmConsensusStateAny, err := codectypes.NewAnyWithValue(wasmConsensusState)
	if err != nil {
		return errors.Wrapf(err, "failed to pack message")
	}
	wasmConsensusStateJson, err := cosmosCodec.MarshalJSON(wasmConsensusStateAny)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal migrate client message")
	}

	fmt.Println("Client state:", string(wasmClientStateJson))
	fmt.Println("Consensus state:", string(wasmConsensusStateJson))

	return nil
}

func printEthClientCreation(createClientTxBz []byte) error {
	contractAbi, err := abi.JSON(strings.NewReader(sp1ics07tendermint.ContractMetaData.ABI))
	if err != nil {
		return errors.Wrapf(err, "failed to parse contract ABI")
	}
	constructor := contractAbi.Constructor

	// if this fails, the contract bytecode might have changed and this needs to be updated (.bytecode (sans 0x) chars/2)
	args, err := constructor.Inputs.Unpack(createClientTxBz[sp1ICS07TendermintContractBytecodeLength:])
	if err != nil {
		return errors.Wrapf(err, "failed to unpack constructor arguments")
	}

	updateClientProgramVkey := args[0].([32]byte)
	membershipProgramVkey := args[1].([32]byte)
	updateClientAndMembershipProgramVkey := args[2].([32]byte)
	misbehaviourProgramVkey := args[3].([32]byte)
	sp1Verifier := args[4].(ethcommon.Address)
	clientState := args[5].([]byte)
	consensusState := args[6].([32]byte)
	// 8th is role manager, but we don't need it for this

	fmt.Printf(`{
  "clientId": "",
  "implementation": "0x0000000000000000000000000000000000000000",
  "verifier": "%s",
  "counterpartyClientId": "SET_ME",
  "merklePrefix": [
	"ibc",
	""
  ],
  "trustedClientState": "%s",
  "trustedConsensusState": "%s",
  "updateClientVkey": "0x%s",
  "membershipVkey": "0x%s",
  "ucAndMembershipVkey": "0x%s",
  "misbehaviourVkey": "0x%s"
}
`,
		sp1Verifier.Hex(),
		hex.EncodeToString(clientState),
		hex.EncodeToString(consensusState[:]),
		hex.EncodeToString(updateClientProgramVkey[:]),
		hex.EncodeToString(membershipProgramVkey[:]),
		hex.EncodeToString(updateClientAndMembershipProgramVkey[:]),
		hex.EncodeToString(misbehaviourProgramVkey[:]),
	)

	return nil
}
