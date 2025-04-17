package cmd

import (
	"encoding/json"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/gogoproto/proto"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/v10/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"

	"github.com/gjermundgaraba/libibc/apis/eurekarelayerapi"
	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/gjermundgaraba/libibc/chains/network"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type InstantiateMsg struct {
	ClientState    []byte `json:"client_state"`
	ConsensusState []byte `json:"consensus_state"`
	Checksum       []byte `json:"checksum"`
}

type WasmMigrateMsg struct {
	InstantiateMsg InstantiateMsg `json:"instantiate_msg"`
}

func clientMigrateMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client-migrate-msg [from-chain-id] [to-chain-id] [client-id] [signer] [key]... [value]...",
		Short: "Create client migrate msg",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cosmosCodec := cosmos.SetupCodec()

			logWriter.AddExtraLogger(func(entry string) {
				cmd.Println(entry)
			})

			net, err := cfg.ToNetwork(ctx, logger, extraGwei)
			if err != nil {
				return errors.Wrap(err, "failed to build network")
			}

			srcChainID := args[0]
			dstChainID := args[1]
			clientID := args[2]
			signer := args[3]
			params := make(map[string]string)
			for i := 4; i < len(args); i += 2 {
				if i+1 >= len(args) {
					return errors.New("missing value for key")
				}

				key := args[i]
				value := args[i+1]
				params[key] = value
			}

			fromChain, err := net.GetChain(srcChainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", srcChainID)
			}

			toChain, err := net.GetChain(dstChainID)
			if err != nil {
				return errors.Wrapf(err, "failed to get chain %s", dstChainID)
			}

			if fromChain.GetChainType() != network.ChainTypeEthereum && toChain.GetChainType() != network.ChainTypeCosmos {
				return errors.New("only Ethereum to Cosmos client migration is supported")
			}

			eurekaClient := eurekarelayerapi.NewClient(logger, cfg.EurekaAPIAddr)

			resp, err := eurekaClient.CreateClient(ctx, srcChainID, dstChainID, params)
			if err != nil {
				return errors.Wrap(err, "failed to create client")
			}

			createClientTxBz := resp.Tx

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

			wasmMigrateMsg := &WasmMigrateMsg{
				InstantiateMsg: InstantiateMsg{
					ClientState:    wasmClientState.Data,
					ConsensusState: wasmConsensusState.Data,
					Checksum:       wasmClientState.Checksum,
				},
			}

			wasmMigrateMsgBz, err := json.Marshal(wasmMigrateMsg)
			if err != nil {
				return errors.Wrapf(err, "failed to marshal migrate client message")
			}

			var msgMigrateContract sdk.Msg
			msgMigrateContract = &ibcwasmtypes.MsgMigrateContract{
				Signer:   signer,
				ClientId: clientID,
				Checksum: wasmClientState.Checksum,
				Msg:      wasmMigrateMsgBz,
			}

			msgMigrateContractProtoMsg, ok := msgMigrateContract.(proto.Message)
			if !ok {
				return errors.New("expected MsgMigrateContract message")
			}

			msgMigrateContractAny, err := codectypes.NewAnyWithValue(msgMigrateContractProtoMsg)
			if err != nil {
				return errors.Wrapf(err, "failed to pack message")
			}
			msgMigrateContractBz, err := cosmosCodec.MarshalJSON(msgMigrateContractAny)
			if err != nil {
				return errors.Wrapf(err, "failed to marshal migrate client message")
			}

			fmt.Println(string(msgMigrateContractBz))

			return nil
		},
	}

	return cmd
}
