package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/v10/types"

	"github.com/gjermundgaraba/libibc/chains/cosmos"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type WasmMigrateMsg2 struct {
	Migration MigrationMsg `json:"migration"`
}

type MigrationMsg struct {
	UpdateForkParameters struct {
		GenesisForkVersion string `json:"genesis_fork_version"`
		GenesisSlot        int    `json:"genesis_slot"`
		Altair             struct {
			Version string `json:"version"`
			Epoch   int    `json:"epoch"`
		} `json:"altair"`
		Bellatrix struct {
			Version string `json:"version"`
			Epoch   int    `json:"epoch"`
		} `json:"bellatrix"`
		Capella struct {
			Version string `json:"version"`
			Epoch   int    `json:"epoch"`
		} `json:"capella"`
		Deneb struct {
			Version string `json:"version"`
			Epoch   int    `json:"epoch"`
		} `json:"deneb"`
		Electra struct {
			Version string `json:"version"`
			Epoch   int    `json:"epoch"`
		} `json:"electra"`
	} `json:"update_fork_parameters"`
}

func clientMigrateMsgCmd2() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client-migrate-msg-2 [client-id] [signer]",
		Short: "Create client migrate msg",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cosmosCodec := cosmos.SetupCodec()

			logWriter.AddExtraLogger(func(entry string) {
				cmd.Println(entry)
			})

			clientID := args[0]
			signer := args[1]

			migrationMsg := MigrationMsg{}

			// Mainnet
			// "fork_parameters":{"genesis_fork_version":"0x00000000","genesis_slot":0,"altair":{"version":"0x01000000","epoch":74240},"bellatrix":{"version":"0x02000000","epoch":144896},"capella":{"version":"0x03000000","epoch":194048},"deneb":{"version":"0x04000000","epoch":269568},"electra":{"version":"0x05000000","epoch":18446744073709551615}}
			migrationMsg.UpdateForkParameters.GenesisForkVersion = "0x00000000"
			migrationMsg.UpdateForkParameters.GenesisSlot = 0
			migrationMsg.UpdateForkParameters.Altair.Version = "0x01000000"
			migrationMsg.UpdateForkParameters.Altair.Epoch = 74240
			migrationMsg.UpdateForkParameters.Bellatrix.Version = "0x02000000"
			migrationMsg.UpdateForkParameters.Bellatrix.Epoch = 144896
			migrationMsg.UpdateForkParameters.Capella.Version = "0x03000000"
			migrationMsg.UpdateForkParameters.Capella.Epoch = 194048
			migrationMsg.UpdateForkParameters.Deneb.Version = "0x04000000"
			migrationMsg.UpdateForkParameters.Deneb.Epoch = 269568
			migrationMsg.UpdateForkParameters.Electra.Version = "0x05000000"
			migrationMsg.UpdateForkParameters.Electra.Epoch = 364032

			// Testnet
			// "fork_parameters":{"genesis_fork_version":"0x90000069","genesis_slot":0,"altair":{"version":"0x90000070","epoch":50},"bellatrix":{"version":"0x90000071","epoch":100},"capella":{"version":"0x90000072","epoch":56832},"deneb":{"version":"0x90000073","epoch":132608},"electra":{"version":"0x90000074","epoch":222464}}
			// migrationMsg.UpdateForkParameters.GenesisForkVersion = "0x90000069"
			// migrationMsg.UpdateForkParameters.GenesisSlot = 0
			// migrationMsg.UpdateForkParameters.Altair.Version = "0x90000070"
			// migrationMsg.UpdateForkParameters.Altair.Epoch = 50
			// migrationMsg.UpdateForkParameters.Bellatrix.Version = "0x90000071"
			// migrationMsg.UpdateForkParameters.Bellatrix.Epoch = 100
			// migrationMsg.UpdateForkParameters.Capella.Version = "0x90000072"
			// migrationMsg.UpdateForkParameters.Capella.Epoch = 56832
			// migrationMsg.UpdateForkParameters.Deneb.Version = "0x90000073"
			// migrationMsg.UpdateForkParameters.Deneb.Epoch = 132608
			// migrationMsg.UpdateForkParameters.Electra.Version = "0x90000074"
			// migrationMsg.UpdateForkParameters.Electra.Epoch = 222464

			wasmMigrateMsg := &WasmMigrateMsg2{
				Migration: migrationMsg,
			}

			wasmMigrateMsgBz, err := json.Marshal(wasmMigrateMsg)
			if err != nil {
				return errors.Wrapf(err, "failed to marshal migrate client message")
			}

			hexChecksum := "b92e9904aab2292916507f0db04b7ab6d024c2fdb57a9d52e6725f69b2e684c1"
			checksumBz, err := hex.DecodeString(hexChecksum)
			if err != nil {
				return errors.Wrapf(err, "failed to decode checksum")
			}

			var msgMigrateContract sdk.Msg
			msgMigrateContract = &ibcwasmtypes.MsgMigrateContract{
				Signer:   signer,
				ClientId: clientID,
				Checksum: checksumBz,
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
