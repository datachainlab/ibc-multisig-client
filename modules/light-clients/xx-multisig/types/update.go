package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/modules/core/exported"
)

// CheckHeaderAndUpdateState checks if the provided header is valid and updates
// the consensus state if appropriate. It returns an error if:
// - the header provided is not parseable to a Multisig header
// - the header epoch does not match the current epoch
// - the header height is not 0
// - the header timestamp is less than the consensus state timestamp
// - the currently registered public key did not provide the update signature
func (cs ClientState) CheckHeaderAndUpdateState(
	ctx sdk.Context, cdc codec.BinaryCodec, clientStore sdk.KVStore,
	header exported.Header,
) (exported.ClientState, exported.ConsensusState, error) {
	smHeader, ok := header.(*Header)
	if !ok {
		return nil, nil, sdkerrors.Wrapf(
			clienttypes.ErrInvalidHeader, "header type %T, expected  %T", header, &Header{},
		)
	}
	consensusState, err := getConsensusState(clientStore, cdc, cs.GetConsensusHeight())
	if err != nil {
		return nil, nil, sdkerrors.Wrapf(
			err, "could not get consensus state from clientstore at height: %s", cs.GetConsensusHeight(),
		)
	}

	if err := checkValidity(cdc, &cs, consensusState, smHeader); err != nil {
		return nil, nil, err
	}

	clientState, consensusState := update(&cs, smHeader)
	return clientState, consensusState, nil
}

// checkValidity checks if the Multisig update signature is valid.
func checkValidity(cdc codec.BinaryCodec, clientState *ClientState, consensusState *ConsensusState, header *Header) error {
	// assert given epoch is current epoch
	if header.Height.RevisionNumber != clientState.Height.RevisionNumber+1 {
		return sdkerrors.Wrapf(
			clienttypes.ErrInvalidHeader,
			"header epoch does not match the client state epoch (%d != %d)", header.Height.RevisionNumber, clientState.Height.RevisionNumber+1,
		)
	}

	if header.Height.RevisionHeight != 1 {
		return sdkerrors.Wrapf(
			clienttypes.ErrInvalidHeader,
			"header height must be 1 (%d)", header.Height.RevisionHeight,
		)
	}

	// assert update timestamp is not less than current consensus state timestamp
	if header.Timestamp < consensusState.Timestamp {
		return sdkerrors.Wrapf(
			clienttypes.ErrInvalidHeader,
			"header timestamp is less than to the consensus state timestamp (%d < %d)", header.Timestamp, consensusState.Timestamp,
		)
	}

	// assert currently registered public key signed over the new public key with correct height
	data, err := HeaderSignBytes(cdc, header)
	if err != nil {
		return err
	}

	sigData, err := UnmarshalSignatureData(cdc, header.Signature)
	if err != nil {
		return err
	}

	publicKey, err := consensusState.GetPubKey()
	if err != nil {
		return err
	}

	if err := VerifySignature(publicKey, data, sigData); err != nil {
		return sdkerrors.Wrap(ErrInvalidHeader, err.Error())
	}

	return nil
}

// update the consensus state to the new public key and an incremented revision number
func update(clientState *ClientState, header *Header) (*ClientState, *ConsensusState) {
	consensusState := &ConsensusState{
		PublicKey:   header.NewPublicKey,
		Diversifier: header.NewDiversifier,
		Timestamp:   header.Timestamp,
	}

	clientState.Height = clienttypes.NewHeight(
		clientState.Height.RevisionNumber+1,
		1,
	)

	return clientState, consensusState
}
