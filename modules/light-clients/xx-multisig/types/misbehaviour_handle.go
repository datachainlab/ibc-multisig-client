package types

import (
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/modules/core/exported"
)

// CheckMisbehaviourAndUpdateState determines whether or not the currently registered
// public key signed over two different messages with the same sequence. If this is true
// the client state is updated to a frozen status.
// NOTE: Misbehaviour is not tracked for previous public keys, a Multisig may update to
// a new public key before the misbehaviour is processed. Therefore, misbehaviour is data
// order processing dependent.
func (cs ClientState) CheckMisbehaviourAndUpdateState(
	ctx sdk.Context,
	cdc codec.BinaryCodec,
	clientStore sdk.KVStore,
	misbehaviour exported.Misbehaviour,
) (exported.ClientState, error) {

	soloMisbehaviour, ok := misbehaviour.(*Misbehaviour)
	if !ok {
		return nil, sdkerrors.Wrapf(
			clienttypes.ErrInvalidClientType,
			"misbehaviour type %T, expected %T", misbehaviour, &Misbehaviour{},
		)
	}

	if soloMisbehaviour.SignatureOne.DataType != HEADER || soloMisbehaviour.SignatureTwo.DataType != HEADER {
		return nil, sdkerrors.Wrap(errors.New("unknown misbehaviour type"), "DataType must be HEADER")
	} else if soloMisbehaviour.SignatureOne.Height.RevisionNumber != soloMisbehaviour.SignatureTwo.Height.RevisionNumber {
		return nil, sdkerrors.Wrap(errors.New("unknown misbehaviour type"), "RevisionNumber mismatch")
	}

	height := clienttypes.NewHeight(soloMisbehaviour.Epoch, 1)
	consensusState, err := getConsensusState(clientStore, cdc, height)
	if err != nil {
		return nil, sdkerrors.Wrapf(
			clienttypes.ErrConsensusStateNotFound,
			"consensus state does not exist for height %s", height,
		)
	}

	// NOTE: a check that the misbehaviour message data are not equal is done by
	// misbehaviour.ValidateBasic which is called by the 02-client keeper.

	// verify first signature
	if err := verifySignatureAndData(cdc, cs, consensusState, soloMisbehaviour, soloMisbehaviour.SignatureOne); err != nil {
		return nil, sdkerrors.Wrap(err, "failed to verify signature one")
	}

	// verify second signature
	if err := verifySignatureAndData(cdc, cs, consensusState, soloMisbehaviour, soloMisbehaviour.SignatureTwo); err != nil {
		return nil, sdkerrors.Wrap(err, "failed to verify signature two")
	}

	cs.FrozenHeight = clienttypes.NewHeight(soloMisbehaviour.Epoch, 1)
	return &cs, nil
}

// verifySignatureAndData verifies that the currently registered public key has signed
// over the provided data and that the data is valid. The data is valid if it can be
// unmarshaled into the specified data type.
func verifySignatureAndData(cdc codec.BinaryCodec, clientState ClientState, consensusState *ConsensusState, misbehaviour *Misbehaviour, sigAndData *SignatureAndData) error {

	// do not check misbehaviour timestamp since we want to allow processing of past misbehaviour

	// ensure data can be unmarshaled to the specified data type
	if _, err := UnmarshalDataByType(cdc, sigAndData.DataType, sigAndData.Data); err != nil {
		return err
	}

	data, err := MisbehaviourSignBytes(
		cdc,
		clienttypes.NewHeight(misbehaviour.Epoch, 1), sigAndData.Timestamp,
		consensusState.Diversifier,
		sigAndData.DataType,
		sigAndData.Data,
	)
	if err != nil {
		return err
	}

	sigData, err := UnmarshalSignatureData(cdc, sigAndData.Signature)
	if err != nil {
		return err
	}

	publicKey, err := consensusState.GetPubKey()
	if err != nil {
		return err
	}

	if err := VerifySignature(publicKey, data, sigData); err != nil {
		return err
	}

	return nil

}
