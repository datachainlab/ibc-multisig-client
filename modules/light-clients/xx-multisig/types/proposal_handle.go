package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/modules/core/exported"
)

func (cs ClientState) CheckSubstituteAndUpdateState(
	ctx sdk.Context, cdc codec.BinaryMarshaler, subjectClientStore,
	_ sdk.KVStore, substituteClient exported.ClientState,
	_ exported.Height,
) (exported.ClientState, error) {
	panic("not implemented")
}
