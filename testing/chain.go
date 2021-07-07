package ibctesting

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/cosmos-sdk/x/staking/teststaking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmtypes "github.com/tendermint/tendermint/types"

	clienttypes "github.com/cosmos/ibc-go/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/modules/core/24-host"
	"github.com/cosmos/ibc-go/modules/core/exported"
	"github.com/cosmos/ibc-go/modules/core/types"
	ibctmtypes "github.com/cosmos/ibc-go/modules/light-clients/07-tendermint/types"

	"github.com/datachainlab/ibc-multisig-client/testing/mock"
	"github.com/datachainlab/ibc-multisig-client/testing/simapp"
)

var _ TestChainI = (*TestMultisigChain)(nil)

type TestChainI interface {
	T() *testing.T
	GetCoordinator() *Coordinator
	GetApp() TestingApp
	GetChainID() string
	GetContext() sdk.Context
	GetLastHeader() *ibctmtypes.Header
	GetSenderAccount() authtypes.AccountI
	GetSimApp() *simapp.SimApp

	//QueryProofAtHeight(key []byte, height int64) ([]byte, clienttypes.Height)
	QueryProof(key []byte) ([]byte, clienttypes.Height)

	//SetCurrentHeaderTime(t time.Time)
	//BeginBlock() abci.ResponseBeginBlock
	//NextBlock()

	SendMsgs(msgs ...sdk.Msg) (*sdk.Result, error)

	GetClientState(clientID string) exported.ClientState
	GetConsensusState(clientID string, height exported.Height) (exported.ConsensusState, bool)

	GetPrefix() commitmenttypes.MerklePrefix
	//ConstructUpdateClientHeader(counterparty TestChainI, clientID string, trustedHeight clienttypes.Height) (*ibctmtypes.Header, error)

	GetChannelCapability(portID, channelID string) *capabilitytypes.Capability
}

// TestMultisigChain is a testing struct that wraps a simapp with the last TM Header, the current ABCI
// header and the validators of the TestMultisigChain. It also contains a field called ChainID. This
// is the clientID that *other* chains use to refer to this TestMultisigChain. The SenderAccount
// is used for delivering transactions through the application state.
// NOTE: the actual application uses an empty chain-id for ease of testing.
type TestMultisigChain struct {
	t *testing.T

	Coordinator *Coordinator
	App         TestingApp
	ChainID     string

	LastHeader    *ibctmtypes.Header // header for last block height committed
	CurrentHeader tmproto.Header     // header for current block height
	QueryServer   types.QueryServer
	TxConfig      client.TxConfig
	Codec         codec.BinaryCodec

	//Vals    *tmtypes.ValidatorSet
	//Signers []tmtypes.PrivValidator

	senderPrivKey cryptotypes.PrivKey
	SenderAccount authtypes.AccountI
}

// NewTestMultisigChain initializes a new TestMultisigChain instance with a single validator set using a
// generated private key. It also creates a sender account to be used for delivering transactions.
//
// The first block height is committed to state in order to allow for client creations on
// counterparty chains. The TestMultisigChain will return with a block height starting at 2.
//
// Time management is handled by the Coordinator in order to ensure synchrony between chains.
// Each update of any chain increments the block header time for all chains by 5 seconds.
func NewTestMultisigChain(t *testing.T, coord *Coordinator, chainID string) TestChainI {
	// generate validator private/public key
	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	// create validator set with single validator
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	//signers := []tmtypes.PrivValidator{privVal}

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(100000000000000))),
	}

	app := SetupWithGenesis(t, valSet, []authtypes.GenesisAccount{acc}, balance)

	// create current header and call begin block
	//header := tmproto.Header{
	//	ChainID: chainID,
	//	Height:  1,
	//	Time:    coord.CurrentTime.UTC(),
	//}

	txConfig := app.GetTxConfig()

	// create an account to send transactions from
	chain := &TestMultisigChain{
		t:           t,
		Coordinator: coord,
		ChainID:     chainID,
		App:         app,
		//CurrentHeader: header,
		QueryServer:   app.GetIBCKeeper(),
		TxConfig:      txConfig,
		Codec:         app.AppCodec(),
		senderPrivKey: senderPrivKey,
		SenderAccount: acc,
	}

	coord.CommitBlock(chain)

	return chain
}

func (chain *TestMultisigChain) T() *testing.T {
	return chain.t
}

func (chain *TestMultisigChain) GetCoordinator() *Coordinator {
	return chain.Coordinator
}

// GetApp returns the current testing application.
func (chain *TestMultisigChain) GetApp() TestingApp {
	return chain.App
}

func (chain *TestMultisigChain) GetChainID() string {
	return chain.ChainID
}

// GetContext returns the current context for the application.
func (chain *TestMultisigChain) GetContext() sdk.Context {
	return chain.App.GetBaseApp().NewContext(chain.CurrentHeader)
}

func (chain *TestMultisigChain) GetLastHeader() *ibctmtypes.Header {
	return chain.LastHeader
}

func (chain *TestMultisigChain) GetSenderAccount() authtypes.AccountI {
	return chain.SenderAccount
}

// GetSimApp returns the SimApp to allow usage ofnon-interface fields.
// CONTRACT: This function should not be called by third parties implementing
// their own SimApp.
func (chain *TestMultisigChain) GetSimApp() *simapp.SimApp {
	app, ok := chain.App.(*simapp.SimApp)
	require.True(chain.t, ok)

	return app
}

// QueryProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (chain *TestMultisigChain) QueryProof(key []byte) ([]byte, clienttypes.Height) {
	req := abci.RequestQuery{
		Path:  fmt.Sprintf("store/%s/key", host.StoreKey),
		Data:  key,
		Prove: true,
	}
	res := chain.App.Query(req)
	fmt.Print(res)
	//prefix := commitmenttypes.NewMerklePrefix(chain.App.GetIBCKeeper().ConnectionKeeper.GetCommitmentPrefix().Bytes())
	//path, err := commitmenttypes.ApplyPrefix(prefix, string(req.Data))
	//require.NoError(chain.t, err)

	//merkleProof, err := commitmenttypes.ConvertProofs(res.ProofOps)
	//require.NoError(chain.t, err)
	//tsd := &multisigtypes.TimestampedSignatureData{
	//	SignatureData: res.Signature,
	//	Timestamp:     res.Timestamp,
	//}
	//
	//sig, err := chain.App.AppCodec().Marshal(tsd)
	//require.NoError(chain.t, err)
	//revision := clienttypes.ParseChainID(chain.ChainID)

	//return sig, clienttypes.NewHeight(revision, uint64(res.Height)+1)
	return nil, clienttypes.Height{}
}

//func (chain TestMultisigChain) QueryCommitment(req *abci.ResponseQuery) (*QueryierQueryCommitmentResponse, error) {
//	prefix := commitmenttypes.NewMerklePrefix(chain.App.GetIBCKeeper().ConnectionKeeper.GetCommitmentPrefix().Bytes())
//	path, err := commitmenttypes.ApplyPrefix(prefix, string(req.Request.Data))
//	if err != nil {
//		return nil, err
//	}
//
//	var value []byte
//	dt := solomachinetypes.DataType(req.DataType)
//	switch dt {
//	case solomachinetypes.CONNECTION:
//		var conn connectiontypes.ConnectionEnd
//		app.AppCodec().MustUnmarshalBinaryBare(res.Value, &conn)
//		value, err = solomachinetypes.ConnectionStateDataBytes(
//			app.AppCodec(),
//			path,
//			conn,
//		)
//	case solomachinetypes.CLIENT:
//		clientState, err := clienttypes.UnmarshalClientState(app.AppCodec(), res.Value)
//		if err != nil {
//			return nil, err
//		}
//		value, err = solomachinetypes.ClientStateDataBytes(
//			app.AppCodec(),
//			path,
//			clientState,
//		)
//	case solomachinetypes.CHANNEL:
//		var channel channeltypes.Channel
//		app.AppCodec().MustUnmarshalBinaryBare(res.Value, &channel)
//		value, err = solomachinetypes.ChannelStateDataBytes(
//			app.AppCodec(),
//			path,
//			channel,
//		)
//	case solomachinetypes.PACKETCOMMITMENT:
//		value, err = solomachinetypes.PacketCommitmentDataBytes(
//			app.AppCodec(),
//			path,
//			res.Value,
//		)
//	case solomachinetypes.PACKETACKNOWLEDGEMENT:
//		value, err = solomachinetypes.PacketAcknowledgementDataBytes(
//			app.AppCodec(),
//			path,
//			res.Value,
//		)
//	default:
//		panic(dt)
//	}
//	if err != nil {
//		return nil, err
//	}
//
//	signBytesData := s.solomachine.MakeSignBytes(tx, solomachinetypes.DataType(req.DataType), value)
//	signBytes, err := app.AppCodec().MarshalBinaryBare(signBytesData)
//	if err != nil {
//		return nil, err
//	}
//
//	sigData, err := s.solomachine.GenerateSignature(signBytes)
//	if err != nil {
//		return nil, err
//	}
//	sig, err := app.AppCodec().MarshalBinaryBare(sigData)
//	if err != nil {
//		return nil, err
//	}
//
//	err = s.solomachine.MakeCommitment(tx, solomachine.Commitment{
//		Sequence:  signBytesData.Sequence,
//		Timestamp: signBytesData.Timestamp,
//		SignBytes: signBytes,
//		Signature: sig,
//	})
//	if err != nil { // TODO if got any error, rollback tx
//		panic(err)
//	}
//	if err := txm.Commit(); err != nil {
//		return nil, err
//	}
//	return &QueryierQueryCommitmentResponse{SignBytes: signBytes, Signature: sig, Sequence: signBytesData.Sequence, Timestamp: signBytesData.Timestamp}, nil
//}

// QueryUpgradeProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (chain *TestMultisigChain) QueryUpgradeProof(key []byte, height uint64) ([]byte, clienttypes.Height) {
	res := chain.App.Query(abci.RequestQuery{
		Path:   "store/upgrade/key",
		Height: int64(height - 1),
		Data:   key,
		Prove:  true,
	})

	merkleProof, err := commitmenttypes.ConvertProofs(res.ProofOps)
	require.NoError(chain.t, err)

	proof, err := chain.App.AppCodec().Marshal(&merkleProof)
	require.NoError(chain.t, err)

	revision := clienttypes.ParseChainID(chain.ChainID)

	// proof height + 1 is returned as the proof created corresponds to the height the proof
	// was created in the IAVL tree. Tendermint and subsequently the clients that rely on it
	// have heights 1 above the IAVL tree. Thus we return proof height + 1
	return proof, clienttypes.NewHeight(revision, uint64(res.Height+1))
}

// QueryConsensusStateProof performs an abci query for a consensus state
// stored on the given clientID. The proof and consensusHeight are returned.
func (chain *TestMultisigChain) QueryConsensusStateProof(clientID string) ([]byte, clienttypes.Height) {
	clientState := chain.GetClientState(clientID)

	consensusHeight := clientState.GetLatestHeight().(clienttypes.Height)
	consensusKey := host.FullConsensusStateKey(clientID, consensusHeight)
	proofConsensus, _ := chain.QueryProof(consensusKey)

	return proofConsensus, consensusHeight
}

// SendMsgs delivers a transaction through the application. It updates the senders sequence
// number and updates the TestMultisigChain's headers. It returns the result and error if one
// occurred.
func (chain *TestMultisigChain) SendMsgs(msgs ...sdk.Msg) (*sdk.Result, error) {
	r, err := simapp.SignAndDeliver(
		chain.t,
		chain.TxConfig,
		chain.App.GetBaseApp(),
		chain.GetContext().BlockHeader(),
		msgs,
		chain.ChainID,
		[]uint64{chain.SenderAccount.GetAccountNumber()},
		[]uint64{chain.SenderAccount.GetSequence()},
		true, true, chain.senderPrivKey,
	)
	if err != nil {
		return nil, err
	}

	// increment sequence for successful transaction execution
	require.NoError(chain.T(), chain.SenderAccount.SetSequence(chain.SenderAccount.GetSequence()+1))
	return r, nil
}

// GetClientState retrieves the client state for the provided clientID. The client is
// expected to exist otherwise testing will fail.
func (chain *TestMultisigChain) GetClientState(clientID string) exported.ClientState {
	clientState, found := chain.App.GetIBCKeeper().ClientKeeper.GetClientState(chain.GetContext(), clientID)
	require.True(chain.t, found)

	return clientState
}

// GetConsensusState retrieves the consensus state for the provided clientID and height.
// It will return a success boolean depending on if consensus state exists or not.
func (chain *TestMultisigChain) GetConsensusState(clientID string, height exported.Height) (exported.ConsensusState, bool) {
	return chain.App.GetIBCKeeper().ClientKeeper.GetClientConsensusState(chain.GetContext(), clientID, height)
}

// GetValsAtHeight will return the validator set of the chain at a given height. It will return
// a success boolean depending on if the validator set exists or not at that height.
func (chain *TestMultisigChain) GetValsAtHeight(height int64) (*tmtypes.ValidatorSet, bool) {
	histInfo, ok := chain.App.GetStakingKeeper().GetHistoricalInfo(chain.GetContext(), height)
	if !ok {
		return nil, false
	}

	valSet := stakingtypes.Validators(histInfo.Valset)

	tmValidators, err := teststaking.ToTmValidators(valSet, sdk.DefaultPowerReduction)
	if err != nil {
		panic(err)
	}
	return tmtypes.NewValidatorSet(tmValidators), true
}

// GetAcknowledgement retrieves an acknowledgement for the provided packet. If the
// acknowledgement does not exist then testing will fail.
func (chain *TestMultisigChain) GetAcknowledgement(packet exported.PacketI) []byte {
	ack, found := chain.App.GetIBCKeeper().ChannelKeeper.GetPacketAcknowledgement(chain.GetContext(), packet.GetDestPort(), packet.GetDestChannel(), packet.GetSequence())
	require.True(chain.t, found)

	return ack
}

// GetPrefix returns the prefix for used by a chain in connection creation
func (chain *TestMultisigChain) GetPrefix() commitmenttypes.MerklePrefix {
	return commitmenttypes.NewMerklePrefix(chain.App.GetIBCKeeper().ConnectionKeeper.GetCommitmentPrefix().Bytes())
}

func (chain *TestMultisigChain) ConstructUpdateClientHeader(counterparty TestChainI, clientID string, trustedHeight clienttypes.Height) (*ibctmtypes.Header, error) {
	return chain.LastHeader, nil
	//header := chain.LastHeader
	//// Relayer must query for LatestHeight on client to get TrustedHeight if the trusted height is not set
	//if trustedHeight.IsZero() {
	//	trustedHeight = counterparty.GetClientState(clientID).GetLatestHeight().(clienttypes.Height)
	//}
	//var (
	//	tmTrustedVals *tmtypes.ValidatorSet
	//	ok            bool
	//)
	//// Once we get TrustedHeight from client, we must query the validators from the counterparty chain
	//// If the LatestHeight == LastHeader.Height, then TrustedValidators are current validators
	//// If LatestHeight < LastHeader.Height, we can query the historical validator set from HistoricalInfo
	//if trustedHeight == chain.LastHeader.GetHeight() {
	//	tmTrustedVals = chain.Vals
	//} else {
	//	// NOTE: We need to get validators from counterparty at height: trustedHeight+1
	//	// since the last trusted validators for a header at height h
	//	// is the NextValidators at h+1 committed to in header h by
	//	// NextValidatorsHash
	//	tmTrustedVals, ok = chain.GetValsAtHeight(int64(trustedHeight.RevisionHeight + 1))
	//	if !ok {
	//		return nil, sdkerrors.Wrapf(ibctmtypes.ErrInvalidHeaderHeight, "could not retrieve trusted validators at trustedHeight: %d", trustedHeight)
	//	}
	//}
	//// inject trusted fields into last header
	//// for now assume revision number is 0
	//header.TrustedHeight = trustedHeight
	//
	//trustedVals, err := tmTrustedVals.ToProto()
	//if err != nil {
	//	return nil, err
	//}
	//header.TrustedValidators = trustedVals
	//
	//return header, nil
}

// ExpireClient fast forwards the chain's block time by the provided amount of time which will
// expire any clients with a trusting period less than or equal to this amount of time.
func (chain *TestMultisigChain) ExpireClient(amount time.Duration) {
	chain.Coordinator.IncrementTimeBy(amount)
}

// CurrentTMClientHeader creates a TM header using the current header parameters
// on the chain. The trusted fields in the header are set to nil.
//func (chain *TestMultisigChain) CurrentTMClientHeader() *ibctmtypes.Header {
//	return chain.CreateTMClientHeader(chain.ChainID, chain.CurrentHeader.Height, clienttypes.Height{}, chain.CurrentHeader.Time, chain.Vals, nil, chain.Signers)
//}

// CreateTMClientHeader creates a TM header to update the TM client. Args are passed in to allow
// caller flexibility to use params that differ from the chain.
//func (chain *TestMultisigChain) CreateTMClientHeader(chainID string, blockHeight int64, trustedHeight clienttypes.Height, timestamp time.Time, tmValSet, tmTrustedVals *tmtypes.ValidatorSet, signers []tmtypes.PrivValidator) *ibctmtypes.Header {
//	var (
//		valSet      *tmproto.ValidatorSet
//		trustedVals *tmproto.ValidatorSet
//	)
//	require.NotNil(chain.t, tmValSet)
//
//	vsetHash := tmValSet.Hash()
//
//	tmHeader := tmtypes.Header{
//		Version:            tmprotoversion.Consensus{Block: tmversion.BlockProtocol, App: 2},
//		ChainID:            chainID,
//		Height:             blockHeight,
//		Time:               timestamp,
//		LastBlockID:        MakeBlockID(make([]byte, tmhash.Size), 10_000, make([]byte, tmhash.Size)),
//		LastCommitHash:     chain.App.LastCommitID().Hash,
//		DataHash:           tmhash.Sum([]byte("data_hash")),
//		ValidatorsHash:     vsetHash,
//		NextValidatorsHash: vsetHash,
//		ConsensusHash:      tmhash.Sum([]byte("consensus_hash")),
//		AppHash:            chain.CurrentHeader.AppHash,
//		LastResultsHash:    tmhash.Sum([]byte("last_results_hash")),
//		EvidenceHash:       tmhash.Sum([]byte("evidence_hash")),
//		ProposerAddress:    tmValSet.Proposer.Address, //nolint:staticcheck
//	}
//	hhash := tmHeader.Hash()
//	blockID := MakeBlockID(hhash, 3, tmhash.Sum([]byte("part_set")))
//	voteSet := tmtypes.NewVoteSet(chainID, blockHeight, 1, tmproto.PrecommitType, tmValSet)
//
//	commit, err := tmtypes.MakeCommit(blockID, blockHeight, 1, voteSet, signers, timestamp)
//	require.NoError(chain.t, err)
//
//	signedHeader := &tmproto.SignedHeader{
//		Header: tmHeader.ToProto(),
//		Commit: commit.ToProto(),
//	}
//
//	if tmValSet != nil {
//		valSet, err = tmValSet.ToProto()
//		if err != nil {
//			panic(err)
//		}
//	}
//
//	if tmTrustedVals != nil {
//		trustedVals, err = tmTrustedVals.ToProto()
//		if err != nil {
//			panic(err)
//		}
//	}
//
//	// The trusted fields may be nil. They may be filled before relaying messages to a client.
//	// The relayer is responsible for querying client and injecting appropriate trusted fields.
//	return &ibctmtypes.Header{
//		SignedHeader:      signedHeader,
//		ValidatorSet:      valSet,
//		TrustedHeight:     trustedHeight,
//		TrustedValidators: trustedVals,
//	}
//}

// CreateSortedSignerArray takes two PrivValidators, and the corresponding Validator structs
// (including voting power). It returns a signer array of PrivValidators that matches the
// sorting of ValidatorSet.
// The sorting is first by .VotingPower (descending), with secondary index of .Address (ascending).
func CreateSortedSignerArray(altPrivVal, suitePrivVal tmtypes.PrivValidator,
	altVal, suiteVal *tmtypes.Validator) []tmtypes.PrivValidator {

	switch {
	case altVal.VotingPower > suiteVal.VotingPower:
		return []tmtypes.PrivValidator{altPrivVal, suitePrivVal}
	case altVal.VotingPower < suiteVal.VotingPower:
		return []tmtypes.PrivValidator{suitePrivVal, altPrivVal}
	default:
		if bytes.Compare(altVal.Address, suiteVal.Address) == -1 {
			return []tmtypes.PrivValidator{altPrivVal, suitePrivVal}
		}
		return []tmtypes.PrivValidator{suitePrivVal, altPrivVal}
	}
}

// CreatePortCapability binds and claims a capability for the given portID if it does not
// already exist. This function will fail testing on any resulting error.
// NOTE: only creation of a capbility for a transfer or mock port is supported
// Other applications must bind to the port in InitGenesis or modify this code.
func (chain *TestMultisigChain) CreatePortCapability(scopedKeeper capabilitykeeper.ScopedKeeper, portID string) {
	// check if the portId is already binded, if not bind it
	_, ok := chain.App.GetScopedIBCKeeper().GetCapability(chain.GetContext(), host.PortPath(portID))
	if !ok {
		// create capability using the IBC capability keeper
		cap, err := chain.App.GetScopedIBCKeeper().NewCapability(chain.GetContext(), host.PortPath(portID))
		require.NoError(chain.t, err)

		// claim capability using the scopedKeeper
		err = scopedKeeper.ClaimCapability(chain.GetContext(), cap, host.PortPath(portID))
		require.NoError(chain.t, err)
	}

	//chain.App.Commit()
	//
	//chain.NextBlock()
}

// GetPortCapability returns the port capability for the given portID. The capability must
// exist, otherwise testing will fail.
func (chain *TestMultisigChain) GetPortCapability(portID string) *capabilitytypes.Capability {
	cap, ok := chain.App.GetScopedIBCKeeper().GetCapability(chain.GetContext(), host.PortPath(portID))
	require.True(chain.t, ok)

	return cap
}

// CreateChannelCapability binds and claims a capability for the given portID and channelID
// if it does not already exist. This function will fail testing on any resulting error. The
// scoped keeper passed in will claim the new capability.
func (chain *TestMultisigChain) CreateChannelCapability(scopedKeeper capabilitykeeper.ScopedKeeper, portID, channelID string) {
	capName := host.ChannelCapabilityPath(portID, channelID)
	// check if the portId is already binded, if not bind it
	_, ok := chain.App.GetScopedIBCKeeper().GetCapability(chain.GetContext(), capName)
	if !ok {
		cap, err := chain.App.GetScopedIBCKeeper().NewCapability(chain.GetContext(), capName)
		require.NoError(chain.t, err)
		err = scopedKeeper.ClaimCapability(chain.GetContext(), cap, capName)
		require.NoError(chain.t, err)
	}

	//chain.App.Commit()
	//
	//chain.NextBlock()
}

// GetChannelCapability returns the channel capability for the given portID and channelID.
// The capability must exist, otherwise testing will fail.
func (chain *TestMultisigChain) GetChannelCapability(portID, channelID string) *capabilitytypes.Capability {
	cap, ok := chain.App.GetScopedIBCKeeper().GetCapability(chain.GetContext(), host.ChannelCapabilityPath(portID, channelID))
	require.True(chain.t, ok)

	return cap
}

func (chain *TestMultisigChain) SetCurrentHeaderTime(t time.Time) {
	chain.CurrentHeader.Time = t
}
