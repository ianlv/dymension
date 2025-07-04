package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterCodec registers the necessary x/incentives interfaces and concrete types on the provided
// LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateGauge{}, "incentives/CreateGauge", nil)
	cdc.RegisterConcrete(&MsgAddToGauge{}, "incentives/AddToGauge", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "incentives/UpdateParams", nil)
	cdc.RegisterConcrete(Params{}, "incentives/Params", nil)
}

// RegisterInterfaces registers interfaces and implementations of the incentives module.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgCreateGauge{},
		&MsgAddToGauge{},
		&MsgUpdateParams{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
