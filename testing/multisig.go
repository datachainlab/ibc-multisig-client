package ibctesting

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/modules/core/24-host"
	"github.com/cosmos/ibc-go/modules/core/exported"
	"github.com/stretchr/testify/require"

	multisigtypes "github.com/datachainlab/ibc-multisig-client/modules/light-clients/xx-multisig/types"
)

// Multisig is a testing helper used to simulate a counterparty
// multisig client.
type Multisig struct {
	t *testing.T

	cdc         codec.BinaryCodec
	PrivateKeys []cryptotypes.PrivKey // keys used for signing
	PublicKeys  []cryptotypes.PubKey  // keys used for generating solo machine pub key
	PublicKey   cryptotypes.PubKey    // key used for verification

	RevisionNumber uint64
	RevisionHeight uint64

	AllowUpdateAfterProposal bool
	Time                     uint64
	Diversifier              string
}

// NewMultisig returns a new multisig instance with an `nKeys` amount of
// generated private/public key pairs and a sequence starting at 1. If nKeys
// is greater than 1 then a multisig public key is used.
func NewMultisig(t *testing.T, cdc codec.BinaryCodec, diversifier string, nKeys uint64) *Multisig {
	privKeys, pubKeys, pk := GenerateMultisigKeys(t, nKeys)

	return &Multisig{
		t:                        t,
		cdc:                      cdc,
		PrivateKeys:              privKeys,
		PublicKeys:               pubKeys,
		RevisionNumber:           0,
		RevisionHeight:           1,
		AllowUpdateAfterProposal: false,
		PublicKey:                pk,
		Time:                     10,
		Diversifier:              diversifier,
	}
}

func GenerateMultisigKeys(t *testing.T, n uint64) ([]cryptotypes.PrivKey, []cryptotypes.PubKey, cryptotypes.PubKey) {
	require.Greater(t, n, uint64(1), "make two or more keys")

	privKeys := make([]cryptotypes.PrivKey, n)
	pubKeys := make([]cryptotypes.PubKey, n)
	for i := uint64(0); i < n; i++ {
		privKeys[i] = secp256k1.GenPrivKey()
		pubKeys[i] = privKeys[i].PubKey()
	}

	var pk cryptotypes.PubKey = kmultisig.NewLegacyAminoPubKey(int(n), pubKeys)

	return privKeys, pubKeys, pk
}

// ClientState returns a new solo machine ClientState instance. Default usage does not allow update
// after governance proposal
func (ms *Multisig) ClientState() *multisigtypes.ClientState {
	return multisigtypes.NewClientState(
		clienttypes.NewHeight(ms.RevisionNumber, ms.RevisionHeight),
		ms.AllowUpdateAfterProposal,
	)
}

// ConsensusState returns a new solo machine ConsensusState instance
func (ms *Multisig) ConsensusState() *multisigtypes.ConsensusState {
	publicKey, err := codectypes.NewAnyWithValue(ms.PublicKey)
	require.NoError(ms.t, err)

	return &multisigtypes.ConsensusState{
		PublicKey:   publicKey,
		Diversifier: ms.Diversifier,
		Timestamp:   ms.Time,
	}
}

// GetHeight returns an exported.Height with Sequence as RevisionHeight
func (ms *Multisig) GetHeight() exported.Height {
	return clienttypes.NewHeight(ms.RevisionNumber, ms.RevisionHeight)
}

// CreateHeader generates a new private/public key pair and creates the
// necessary signature to construct a valid solo machine header.
func (ms *Multisig) CreateHeader() *multisigtypes.Header {
	// generate new private keys and signature for header
	newPrivKeys, newPubKeys, newPubKey := GenerateKeys(ms.t, uint64(len(ms.PrivateKeys)))

	publicKey, err := codectypes.NewAnyWithValue(newPubKey)
	require.NoError(ms.t, err)

	data := &multisigtypes.HeaderData{
		NewPubKey:      publicKey,
		NewDiversifier: ms.Diversifier,
	}

	dataBz, err := ms.cdc.Marshal(data)
	require.NoError(ms.t, err)

	height := clienttypes.NewHeight(ms.RevisionNumber, ms.RevisionHeight)

	signBytes := &multisigtypes.SignBytes{
		Height:      height,
		Timestamp:   ms.Time,
		Diversifier: ms.Diversifier,
		DataType:    multisigtypes.HEADER,
		Data:        dataBz,
	}

	bz, err := ms.cdc.Marshal(signBytes)
	require.NoError(ms.t, err)

	sig := ms.GenerateSignature(bz)

	header := &multisigtypes.Header{
		Height:         height,
		Timestamp:      ms.Time,
		Signature:      sig,
		NewPublicKey:   publicKey,
		NewDiversifier: ms.Diversifier,
	}

	// assumes successful header update
	//ms.RevisionHeight++
	ms.PrivateKeys = newPrivKeys
	ms.PublicKeys = newPubKeys
	ms.PublicKey = newPubKey

	return header
}

// CreateMisbehaviour constructs testing misbehaviour for the solo machine client
// by signing over two different data bytes at the same sequence.
func (ms *Multisig) CreateMisbehaviour(clientID string) *multisigtypes.Misbehaviour {
	path := ms.GetClientStatePath("counterparty")
	dataOne, err := multisigtypes.ClientStateDataBytes(ms.cdc, path, ms.ClientState())
	require.NoError(ms.t, err)

	path = ms.GetConsensusStatePath("counterparty", clienttypes.NewHeight(0, 1))
	dataTwo, err := multisigtypes.ConsensusStateDataBytes(ms.cdc, path, ms.ConsensusState())
	require.NoError(ms.t, err)

	height := clienttypes.NewHeight(ms.RevisionNumber, ms.RevisionHeight)

	signBytes := &multisigtypes.SignBytes{
		Height:      height,
		Timestamp:   ms.Time,
		Diversifier: ms.Diversifier,
		DataType:    multisigtypes.CLIENT,
		Data:        dataOne,
	}

	bz, err := ms.cdc.Marshal(signBytes)
	require.NoError(ms.t, err)

	sig := ms.GenerateSignature(bz)
	signatureOne := multisigtypes.SignatureAndData{
		Signature: sig,
		DataType:  multisigtypes.CLIENT,
		Data:      dataOne,
		Timestamp: ms.Time,
	}

	// misbehaviour signaturess can have different timestamps
	ms.Time++

	signBytes = &multisigtypes.SignBytes{
		Height:      height,
		Timestamp:   ms.Time,
		Diversifier: ms.Diversifier,
		DataType:    multisigtypes.CONSENSUS,
		Data:        dataTwo,
	}

	bz, err = ms.cdc.Marshal(signBytes)
	require.NoError(ms.t, err)

	sig = ms.GenerateSignature(bz)
	signatureTwo := multisigtypes.SignatureAndData{
		Signature: sig,
		DataType:  multisigtypes.CONSENSUS,
		Data:      dataTwo,
		Timestamp: ms.Time,
	}

	return &multisigtypes.Misbehaviour{
		ClientId:     clientID,
		Epoch:        ms.RevisionNumber,
		SignatureOne: &signatureOne,
		SignatureTwo: &signatureTwo,
	}
}

// GenerateSignature uses the stored private keys to generate a signature
// over the sign bytes with each key. If the amount of keys is greater than
// 1 then a multisig data type is returned.
func (ms *Multisig) GenerateSignature(signBytes []byte) []byte {
	sigs := make([]signing.SignatureData, len(ms.PrivateKeys))
	for i, key := range ms.PrivateKeys {
		sig, err := key.Sign(signBytes)
		require.NoError(ms.t, err)

		sigs[i] = &signing.SingleSignatureData{
			Signature: sig,
		}
	}

	var sigData signing.SignatureData
	if len(sigs) == 1 {
		// single public key
		sigData = sigs[0]
	} else {
		// generate multi signature data
		multiSigData := multisig.NewMultisig(len(sigs))
		for i, sig := range sigs {
			multisig.AddSignature(multiSigData, sig, i)
		}

		sigData = multiSigData
	}

	protoSigData := signing.SignatureDataToProto(sigData)
	bz, err := ms.cdc.Marshal(protoSigData)
	require.NoError(ms.t, err)

	return bz
}

// GetClientStatePath returns the commitment path for the client state.
func (ms *Multisig) GetClientStatePath(counterpartyClientIdentifier string) commitmenttypes.MerklePath {
	path, err := commitmenttypes.ApplyPrefix(prefix, commitmenttypes.NewMerklePath(host.FullClientStatePath(counterpartyClientIdentifier)))
	require.NoError(ms.t, err)

	return path
}

// GetConsensusStatePath returns the commitment path for the consensus state.
func (ms *Multisig) GetConsensusStatePath(counterpartyClientIdentifier string, consensusHeight exported.Height) commitmenttypes.MerklePath {
	path, err := commitmenttypes.ApplyPrefix(prefix, commitmenttypes.NewMerklePath(host.FullConsensusStatePath(counterpartyClientIdentifier, consensusHeight)))
	require.NoError(ms.t, err)

	return path
}

// GetConnectionStatePath returns the commitment path for the connection state.
func (ms *Multisig) GetConnectionStatePath(connID string) commitmenttypes.MerklePath {
	connectionPath := commitmenttypes.NewMerklePath(host.ConnectionPath(connID))
	path, err := commitmenttypes.ApplyPrefix(prefix, connectionPath)
	require.NoError(ms.t, err)

	return path
}

// GetChannelStatePath returns the commitment path for that channel state.
func (ms *Multisig) GetChannelStatePath(portID, channelID string) commitmenttypes.MerklePath {
	channelPath := commitmenttypes.NewMerklePath(host.ChannelPath(portID, channelID))
	path, err := commitmenttypes.ApplyPrefix(prefix, channelPath)
	require.NoError(ms.t, err)

	return path
}

// GetPacketCommitmentPath returns the commitment path for a packet commitment.
func (ms *Multisig) GetPacketCommitmentPath(portID, channelID string) commitmenttypes.MerklePath {
	commitmentPath := commitmenttypes.NewMerklePath(host.PacketCommitmentPath(portID, channelID, ms.RevisionNumber))
	path, err := commitmenttypes.ApplyPrefix(prefix, commitmentPath)
	require.NoError(ms.t, err)

	return path
}

// GetPacketAcknowledgementPath returns the commitment path for a packet acknowledgement.
func (ms *Multisig) GetPacketAcknowledgementPath(portID, channelID string) commitmenttypes.MerklePath {
	ackPath := commitmenttypes.NewMerklePath(host.PacketAcknowledgementPath(portID, channelID, ms.RevisionNumber))
	path, err := commitmenttypes.ApplyPrefix(prefix, ackPath)
	require.NoError(ms.t, err)

	return path
}

// GetPacketReceiptPath returns the commitment path for a packet receipt
// and an absent receipts.
func (ms *Multisig) GetPacketReceiptPath(portID, channelID string) commitmenttypes.MerklePath {
	receiptPath := commitmenttypes.NewMerklePath(host.PacketReceiptPath(portID, channelID, ms.RevisionNumber))
	path, err := commitmenttypes.ApplyPrefix(prefix, receiptPath)
	require.NoError(ms.t, err)

	return path
}

// GetNextSequenceRecvPath returns the commitment path for the next sequence recv counter.
func (ms *Multisig) GetNextSequenceRecvPath(portID, channelID string) commitmenttypes.MerklePath {
	nextSequenceRecvPath := commitmenttypes.NewMerklePath(host.NextSequenceRecvPath(portID, channelID))
	path, err := commitmenttypes.ApplyPrefix(prefix, nextSequenceRecvPath)
	require.NoError(ms.t, err)

	return path
}
