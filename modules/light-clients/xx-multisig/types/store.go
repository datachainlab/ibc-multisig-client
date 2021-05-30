package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	host "github.com/cosmos/ibc-go/modules/core/24-host"
	"github.com/cosmos/ibc-go/modules/core/exported"
)

// sets the client state to the store
func setClientState(store sdk.KVStore, cdc codec.BinaryCodec, clientState exported.ClientState) {
	bz := clienttypes.MustMarshalClientState(cdc, clientState)
	store.Set([]byte(host.KeyClientState), bz)
}

// GetConsensusState retrieves the consensus state from the client prefixed
// store. An error is returned if the consensus state does not exist.
func getConsensusState(store sdk.KVStore, cdc codec.BinaryCodec, height exported.Height) (*ConsensusState, error) {
	bz := store.Get(host.ConsensusStateKey(height))
	if bz == nil {
		return nil, sdkerrors.Wrapf(
			clienttypes.ErrConsensusStateNotFound,
			"consensus state does not exist for height %s", height,
		)
	}

	consensusStateI, err := clienttypes.UnmarshalConsensusState(cdc, bz)
	if err != nil {
		return nil, sdkerrors.Wrapf(clienttypes.ErrInvalidConsensus, "unmarshal error: %v", err)
	}

	consensusState, ok := consensusStateI.(*ConsensusState)
	if !ok {
		return nil, sdkerrors.Wrapf(
			clienttypes.ErrInvalidConsensus,
			"invalid consensus type %T, expected %T", consensusState, &ConsensusState{},
		)
	}

	return consensusState, nil
}

// SetConsensusState stores the consensus state at the given height.
func setConsensusState(clientStore sdk.KVStore, cdc codec.BinaryCodec, consensusState *ConsensusState, height exported.Height) {
	key := host.ConsensusStateKey(height)
	val := clienttypes.MustMarshalConsensusState(cdc, consensusState)
	clientStore.Set(key, val)
}
