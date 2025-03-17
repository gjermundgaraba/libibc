package cosmos

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	ibcwasmtypes "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/v10/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	clienttypesv2 "github.com/cosmos/ibc-go/v10/modules/core/02-client/v2/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	tmclient "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
)

func SetupCodec() codec.Codec {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	authtypes.RegisterInterfaces(interfaceRegistry)
	clienttypes.RegisterInterfaces(interfaceRegistry)
	clienttypesv2.RegisterInterfaces(interfaceRegistry)
	connectiontypes.RegisterInterfaces(interfaceRegistry)
	channeltypes.RegisterInterfaces(interfaceRegistry)
	channeltypesv2.RegisterInterfaces(interfaceRegistry)
	transfertypes.RegisterInterfaces(interfaceRegistry)
	tmclient.RegisterInterfaces(interfaceRegistry)
	ibcwasmtypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	return cdc
}
