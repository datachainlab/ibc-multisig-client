package types

import (
	ics23 "github.com/confio/ics23/go"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/modules/core/24-host"
	"github.com/cosmos/ibc-go/modules/core/exported"
)

const (
	Multisig string = "multisig"
)

var _ exported.ClientState = (*ClientState)(nil)

// NewClientState creates a new ClientState instance.
func NewClientState(latestHeight clienttypes.Height, allowUpdateAfterProposal bool) *ClientState {
	return &ClientState{
		Height:                   latestHeight,
		FrozenHeight:             clienttypes.ZeroHeight(),
		AllowUpdateAfterProposal: allowUpdateAfterProposal,
	}
}

// ClientType is Multisig.
func (cs ClientState) ClientType() string {
	return Multisig
}

// GetLatestHeight returns the latest sequence number.
// Return exported.Height to satisfy ClientState interface
// Revision number is always 0 for a solo-machine.
func (cs ClientState) GetLatestHeight() exported.Height {
	return cs.Height
}

// Status returns the status of the solo machine client.
// The client may be:
// - Active: if frozen sequence is 0
// - Frozen: otherwise solo machine is frozen
func (cs ClientState) Status(_ sdk.Context, _ sdk.KVStore, _ codec.BinaryCodec) exported.Status {
	if !cs.FrozenHeight.IsZero() {
		return exported.Frozen
	}

	return exported.Active
}

// GetFrozenHeight returns the frozen sequence of the client.
// Return exported.Height to satisfy interface
// Revision number is always 0 for a solo-machine
func (cs ClientState) GetFrozenHeight() exported.Height {
	return cs.FrozenHeight
}

// GetProofSpecs returns nil proof specs since client state verification uses signatures.
func (cs ClientState) GetProofSpecs() []*ics23.ProofSpec {
	return nil
}

// Validate performs basic validation of the client state fields.
func (cs ClientState) Validate() error {
	if cs.Height.IsZero() {
		return sdkerrors.Wrap(clienttypes.ErrInvalidClient, "sequence cannot be 0")
	}
	return nil
}

// ZeroCustomFields returns Multisig client state with client-specific fields FrozenSequence,
// and AllowUpdateAfterProposal zeroed out
func (cs ClientState) ZeroCustomFields() exported.ClientState {
	return NewClientState(
		cs.Height, false,
	)
}

// Initialize will check that initial consensus state is equal to the latest consensus state of the initial client.
func (cs ClientState) Initialize(_ sdk.Context, _ codec.BinaryCodec, _ sdk.KVStore, consState exported.ConsensusState) error {
	if cs.Height.RevisionHeight != 1 {
		return sdkerrors.Wrapf(clienttypes.ErrInvalidHeader, "the height must be 1, not %v", cs.Height.RevisionHeight)
	}
	return nil
}

// ExportMetadata is a no-op since Multisig does not store any metadata in client store
func (cs ClientState) ExportMetadata(_ sdk.KVStore) []exported.GenesisMetadata {
	return nil
}

// VerifyUpgradeAndUpdateState returns an error since Multisig client does not support upgrades
func (cs ClientState) VerifyUpgradeAndUpdateState(
	_ sdk.Context, _ codec.BinaryCodec, _ sdk.KVStore,
	_ exported.ClientState, _ exported.ConsensusState, _, _ []byte,
) (exported.ClientState, exported.ConsensusState, error) {
	return nil, nil, sdkerrors.Wrap(clienttypes.ErrInvalidUpgradeClient, "cannot upgrade Multisig client")
}

// VerifyClientState verifies a proof of the client state of the running chain
// stored on the Multisig.
func (cs ClientState) VerifyClientState(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	prefix exported.Prefix,
	counterpartyClientIdentifier string,
	proof []byte,
	clientState exported.ClientState,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	clientPrefixedPath := commitmenttypes.NewMerklePath(host.FullClientStatePath(counterpartyClientIdentifier))
	path, err := commitmenttypes.ApplyPrefix(prefix, clientPrefixedPath)
	if err != nil {
		return err
	}

	signBz, err := ClientStateSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path, clientState)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// VerifyClientConsensusState verifies a proof of the consensus state of the
// running chain stored on the Multisig.
func (cs ClientState) VerifyClientConsensusState(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	counterpartyClientIdentifier string,
	consensusHeight exported.Height,
	prefix exported.Prefix,
	proof []byte,
	consensusState exported.ConsensusState,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	clientPrefixedPath := commitmenttypes.NewMerklePath(host.FullConsensusStatePath(counterpartyClientIdentifier, consensusHeight))
	path, err := commitmenttypes.ApplyPrefix(prefix, clientPrefixedPath)
	if err != nil {
		return err
	}

	signBz, err := ConsensusStateSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path, consensusState)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// VerifyConnectionState verifies a proof of the connection state of the
// specified connection end stored on the target machine.
func (cs ClientState) VerifyConnectionState(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	prefix exported.Prefix,
	proof []byte,
	connectionID string,
	connectionEnd exported.ConnectionI,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	connectionPath := commitmenttypes.NewMerklePath(host.ConnectionPath(connectionID))
	path, err := commitmenttypes.ApplyPrefix(prefix, connectionPath)
	if err != nil {
		return err
	}

	signBz, err := ConnectionStateSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path, connectionEnd)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// VerifyChannelState verifies a proof of the channel state of the specified
// channel end, under the specified port, stored on the target machine.
func (cs ClientState) VerifyChannelState(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	prefix exported.Prefix,
	proof []byte,
	portID,
	channelID string,
	channel exported.ChannelI,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	channelPath := commitmenttypes.NewMerklePath(host.ChannelPath(portID, channelID))
	path, err := commitmenttypes.ApplyPrefix(prefix, channelPath)
	if err != nil {
		return err
	}

	signBz, err := ChannelStateSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path, channel)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// VerifyPacketCommitment verifies a proof of an outgoing packet commitment at
// the specified port, specified channel, and specified sequence.
func (cs ClientState) VerifyPacketCommitment(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	_ uint64,
	_ uint64,
	prefix exported.Prefix,
	proof []byte,
	portID,
	channelID string,
	packetSequence uint64,
	commitmentBytes []byte,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	commitmentPath := commitmenttypes.NewMerklePath(host.PacketCommitmentPath(portID, channelID, packetSequence))
	path, err := commitmenttypes.ApplyPrefix(prefix, commitmentPath)
	if err != nil {
		return err
	}

	signBz, err := PacketCommitmentSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path, commitmentBytes)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// VerifyPacketAcknowledgement verifies a proof of an incoming packet
// acknowledgement at the specified port, specified channel, and specified sequence.
func (cs ClientState) VerifyPacketAcknowledgement(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	_ uint64,
	_ uint64,
	prefix exported.Prefix,
	proof []byte,
	portID,
	channelID string,
	packetSequence uint64,
	acknowledgement []byte,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	ackPath := commitmenttypes.NewMerklePath(host.PacketAcknowledgementPath(portID, channelID, packetSequence))
	path, err := commitmenttypes.ApplyPrefix(prefix, ackPath)
	if err != nil {
		return err
	}

	signBz, err := PacketAcknowledgementSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path, acknowledgement)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// VerifyPacketReceiptAbsence verifies a proof of the absence of an
// incoming packet receipt at the specified port, specified channel, and
// specified sequence.
func (cs ClientState) VerifyPacketReceiptAbsence(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	_ uint64,
	_ uint64,
	prefix exported.Prefix,
	proof []byte,
	portID,
	channelID string,
	packetSequence uint64,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	receiptPath := commitmenttypes.NewMerklePath(host.PacketReceiptPath(portID, channelID, packetSequence))
	path, err := commitmenttypes.ApplyPrefix(prefix, receiptPath)
	if err != nil {
		return err
	}

	signBz, err := PacketReceiptAbsenceSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// VerifyNextSequenceRecv verifies a proof of the next sequence number to be
// received of the specified channel at the specified port.
func (cs ClientState) VerifyNextSequenceRecv(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	_ uint64,
	_ uint64,
	prefix exported.Prefix,
	proof []byte,
	portID,
	channelID string,
	nextSequenceRecv uint64,
) error {
	cons, sigData, timestamp, err := produceVerificationArgs(store, cdc, cs, height, prefix, proof)
	if err != nil {
		return err
	}

	nextSequenceRecvPath := commitmenttypes.NewMerklePath(host.NextSequenceRecvPath(portID, channelID))
	path, err := commitmenttypes.ApplyPrefix(prefix, nextSequenceRecvPath)
	if err != nil {
		return err
	}

	signBz, err := NextSequenceRecvSignBytes(cdc, height.(clienttypes.Height), timestamp, cons.Diversifier, path, nextSequenceRecv)
	if err != nil {
		return err
	}

	if err := VerifySignature(cons.MustGetPubKey(), signBz, sigData); err != nil {
		return err
	}

	cs.Height = cs.Height.Increment().(clienttypes.Height)
	cons.Timestamp = timestamp
	setConsensusState(store, cdc, cons, height)
	setClientState(store, cdc, &cs)
	return nil
}

// produceVerificationArgs perfoms the basic checks on the arguments that are
// shared between the verification functions and returns the public key of the
// consensus state, the unmarshalled proof representing the signature and timestamp
// along with the solo-machine sequence encoded in the proofHeight.
func produceVerificationArgs(
	store sdk.KVStore,
	cdc codec.BinaryCodec,
	cs ClientState,
	height exported.Height,
	prefix exported.Prefix,
	proof []byte,
) (*ConsensusState, signing.SignatureData, uint64, error) {
	if revision := height.GetRevisionNumber(); revision != 0 {
		return nil, nil, 0, sdkerrors.Wrapf(sdkerrors.ErrInvalidHeight, "revision must be 0 for Multisig, got revision-number: %d", revision)
	}

	if prefix == nil {
		return nil, nil, 0, sdkerrors.Wrap(commitmenttypes.ErrInvalidPrefix, "prefix cannot be empty")
	}

	_, ok := prefix.(commitmenttypes.MerklePrefix)
	if !ok {
		return nil, nil, 0, sdkerrors.Wrapf(commitmenttypes.ErrInvalidPrefix, "invalid prefix type %T, expected MerklePrefix", prefix)
	}

	if proof == nil {
		return nil, nil, 0, sdkerrors.Wrap(ErrInvalidProof, "proof cannot be empty")
	}

	timestampedSigData := &TimestampedSignatureData{}
	if err := cdc.Unmarshal(proof, timestampedSigData); err != nil {
		return nil, nil, 0, sdkerrors.Wrapf(err, "failed to unmarshal proof into type %T", timestampedSigData)
	}

	timestamp := timestampedSigData.Timestamp

	if len(timestampedSigData.SignatureData) == 0 {
		return nil, nil, 0, sdkerrors.Wrap(ErrInvalidProof, "signature data cannot be empty")
	}

	sigData, err := UnmarshalSignatureData(cdc, timestampedSigData.SignatureData)
	if err != nil {
		return nil, nil, 0, err
	}

	if cs.GetLatestHeight().GetRevisionNumber() != height.GetRevisionNumber() {
		return nil, nil, 0, sdkerrors.Wrapf(
			sdkerrors.ErrInvalidHeight,
			"client state epoch != proof epoch (%d != %d)", cs.GetLatestHeight().GetRevisionNumber(), height.GetRevisionNumber(),
		)
	}

	cons, err := getConsensusState(store, cdc, height)
	if err != nil {
		return nil, nil, 0, err
	}

	if cons.GetTimestamp() > timestamp {
		return nil, nil, 0, sdkerrors.Wrapf(ErrInvalidProof, "the consensus state timestamp is greater than the signature timestamp (%d >= %d)", cons.GetTimestamp(), timestamp)
	}

	return cons, sigData, timestamp, nil
}
