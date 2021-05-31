---
ics: XX
title: Multisig Client
stage: draft
category: IBC/TAO
kind: instantiation
implements: 2
author: Jun Kimura <jun.kimura@datachain.jp>
created: 2021-05-31
modified: 2021-05-31
---

## Synopsis

This specification document describes a client (verification algorithm) for a solo machine with updateable public keys which implements the [ICS 2](https://github.com/cosmos/ibc/tree/master/spec/core/ics-002-client-semantics) interface. This specification inherits much of [the solo machine client specification](https://github.com/cosmos/ibc/tree/master/spec/client/ics-006-solo-machine-client).

### Motivation

Solo machines — which might be devices such as phones, browsers, or laptops — might like to interface with other machines & replicated ledgers which speak IBC, and they can do so through the uniform client interface.

Solo machine clients are roughly analogous to "implicit accounts" and can be used in lieu of "regular transactions" on a ledger, allowing all transactions to work through the unified interface of IBC.

### Definitions

Functions & terms are as defined in [ICS 2](https://github.com/cosmos/ibc/tree/master/spec/core/ics-002-client-semantics).

### Desired Properties

This specification must satisfy the client interface defined in [ICS 2](https://github.com/cosmos/ibc/tree/master/spec/core/ics-002-client-semantics).

Conceptually, we assume "big table of signatures in the universe" - that signatures produced are public - and incorporate replay protection accordingly.

## Technical Specification

This specification contains implementations for all of the functions defined by [ICS 2](https://github.com/cosmos/ibc/tree/master/spec/core/ics-002-client-semantics).

### Client state

The `ClientState` of a solo machine consists of the current height, frozen_height(if it's frozen).

```typescript
interface ClientState {
  height: Height
  frozen_height: Height
}
```

### Consensus state

The `ConsensusState` of a solo machine consists of the current public key, current diversifier, and timestamp.

The diversifier is an arbitrary string, chosen when the client is created, designed to allow the same public key to be re-used across different solo machine clients (potentially on different chains) without being considered misbehaviour.

```typescript
interface ConsensusState {
  publicKey: PublicKey
  diversifier: string
  timestamp: uint64
}
```

### Height

The height of a multisig client consists of two `uint64`s: the revision number, and the height in the revision.

```typescript
interface Height {
  revision_number: uint64
  revision_height: uint64
}
```

### Headers

`Header`s must only be provided by a solo machine when the machine wishes to update the public key or diversifier.

```typescript
interface Header {
  height: Height
  timestamp: uint64
  signature: Signature
  newPublicKey: PublicKey
  newDiversifier: string
}
```

### Misbehaviour 

`Misbehaviour` for multisig clients consists of a revision and two signatures over conflicting messages in that revision.

```typescript
interface SignatureAndData {
  signature: Signature
  data: []byte
  dataType: uint8
}

interface Misbehaviour {
  revision: uint64
  signatureOne: SignatureAndData
  signatureTwo: SignatureAndData
}
```

### Signatures

Signatures are provided in the `Proof` field of client state verification functions. They include data & a timestamp, which must also be signed over.

```typescript
interface Signature {
  data: []byte
  timestamp: uint64
}
```

### Client initialisation

The multisig client `initialise` function starts an unfrozen client with the initial consensus state.

```typescript
function initialise(clientState: ClientState, consensusState: ConsensusState, height: Height): ClientState {
  assert(height.revision_height == 1)
  set("clients/{identifier}/consensusStates/{height}", consensusState)
  return clientState
}
```

The solo machine client `latestClientHeight` function returns the latest sequence.

```typescript
function latestClientHeight(clientState: ClientState): Height {
  return clientState.height
}
```

### Validity predicate

The solo machine client `checkValidityAndUpdateState` function checks that the currently registered public key has signed over the new public key with the correct sequence.

```typescript
function checkValidityAndUpdateState(
  clientState: ClientState,
  header: Header) {

  consensusState = getCurrentConsensusState()
  // header's revision_number must be indicates next revision number
  assert(header.height.revision_number == clientState.height.revision_number+1)
  // header's revision_height must be 1
  assert(header.height.revision_height == 1)
  assert(header.timestamp >= consensusState.timestamp)
  assert(checkSignature(consensusState.publicKey, header.height, header.diversifier, header.signature))
  clientState.height = header.height
  consensusState.publicKey = header.newPublicKey
  consensusState.diversifier = header.newDiversifier
  consensusState.timestamp = header.timestamp

  set("clients/{identifier}", clientState)
  set("clients/{identifier}/consensusStates/{header.height}", consensusState)
}
```

### Misbehaviour predicate

Any duplicate signature on different messages by the current public key freezes a solo machine client.

```typescript
function checkMisbehaviourAndUpdateState(
  clientState: ClientState,
  misbehaviour: Misbehaviour) {
    h1 = misbehaviour.h1
    h2 = misbehaviour.h2
    // fetch the previously verified consensus state
    consensusState = get("clients/{identifier}/consensusStates/{misbehaviour.revision}")
    pubkey = consensusState.publicKey
    diversifier = consensusState.diversifier
    timestamp = consensusState.timestamp

	// NOTE: Two misbehaviour types are supported:
	// (0). Common conditions:
	//	- two valid sigAndData has same revision_number
	// 1. Two different commitments at same revision_height
	//	- two valid sigAndData has same revision_height
	// 2. Two conflicting updateClient in same revision_number
	//	- two valid sigAndData's type are HEADER

    assert(misbehaviour.revision_number == h1.height.revision_number && misbehaviour.revision_number == h2.height.revision_number)
    
    if (h1.signature.dataType != HEADER && h2.signature.dataType != HEADER) {
        assert(h1.height.revision_height == h1.height.revision_height);
    }

    // assert that signature data is different
    assert(misbehaviour.h1.signature.data !== misbehaviour.h2.signature.data)
    // assert that the signatures validate
    assert(checkSignature(pubkey, misbehaviour.revision, diversifier, misbehaviour.h1.signature.data))
    assert(checkSignature(pubkey, misbehaviour.revision, diversifier, misbehaviour.h2.signature.data))
    // freeze the client
    clientState.frozen = true
}
```

### State verification functions

All solo machine client state verification functions simply check a signature, which must be provided by the solo machine.

Note that value concatenation should be implemented in a state-machine-specific escaped fashion.

```typescript
function verifyClientState(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  clientIdentifier: Identifier,
  counterpartyClientState: ClientState) {
    path = applyPrefix(prefix, "clients/{clientIdentifier}/clientState")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path + counterpartyClientState
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function verifyClientConsensusState(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  clientIdentifier: Identifier,
  consensusStateHeight: uint64,
  targetConsensusState: ConsensusState) {
    path = applyPrefix(prefix, "clients/{clientIdentifier}/consensusState/{consensusStateHeight}")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path + targetConsensusState
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function verifyConnectionState(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  connectionIdentifier: Identifier,
  connectionEnd: ConnectionEnd) {
    path = applyPrefix(prefix, "connection/{connectionIdentifier}")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path + connectionEnd
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function verifyChannelState(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  portIdentifier: Identifier,
  channelIdentifier: Identifier,
  channelEnd: ChannelEnd) {
    path = applyPrefix(prefix, "ports/{portIdentifier}/channels/{channelIdentifier}")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path + channelEnd
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function verifyPacketData(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  portIdentifier: Identifier,
  channelIdentifier: Identifier,
  sequence: uint64,
  data: bytes) {
    path = applyPrefix(prefix, "ports/{portIdentifier}/channels/{channelIdentifier}/packets/{sequence}")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path + data
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function verifyPacketAcknowledgement(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  portIdentifier: Identifier,
  channelIdentifier: Identifier,
  sequence: uint64,
  acknowledgement: bytes) {
    path = applyPrefix(prefix, "ports/{portIdentifier}/channels/{channelIdentifier}/acknowledgements/{sequence}")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path + acknowledgement
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function verifyPacketReceiptAbsence(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  portIdentifier: Identifier,
  channelIdentifier: Identifier,
  sequence: uint64) {
    path = applyPrefix(prefix, "ports/{portIdentifier}/channels/{channelIdentifier}/receipts/{sequence}")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function verifyNextSequenceRecv(
  clientState: ClientState,
  height: uint64,
  prefix: CommitmentPrefix,
  proof: CommitmentProof,
  portIdentifier: Identifier,
  channelIdentifier: Identifier,
  nextSequenceRecv: uint64) {
    path = applyPrefix(prefix, "ports/{portIdentifier}/channels/{channelIdentifier}/nextSequenceRecv")
    consensusState = get("clients/{identifier}/consensusStates/{height}")
    abortTransactionUnless(height.revision_number == clientState.height.revision_number)
    abortTransactionUnless(!clientState.frozen)
    abortTransactionUnless(proof.timestamp >= consensusState.timestamp)
    value = height + consensusState.diversifier + proof.timestamp + path + nextSequenceRecv
    assert(checkSignature(consensusState.pubKey, value, proof.sig))
    updateStateIfHeightIsAdvanced(clientState, height, consensusState, timestamp)
}

function updateStateIfHeightIsAdvanced(
  clientState: ClientState,
  height: uint64,
  consensusState: ConsensusState,
  timestmap: uint64
) {
  if height.GT(clientState.Height) {
    clientState.height = height
    assert(timestamp < consensusState.Timestamp)
    consensusState.Timestamp = timestamp
    set("clients/{identifier}", clientState)
    set("clients/{identifier}/consensusStates/{height}", consensusState)
  }
}
```

### Properties & Invariants

Instantiates the interface defined in [ICS 2](https://github.com/cosmos/ibc/tree/master/spec/core/ics-002-client-semantics).

## Backwards Compatibility

Not applicable.

## Forwards Compatibility

Not applicable. Alterations to the client verification algorithm will require a new client standard.

## Example Implementation

None yet.

## Other Implementations

None at present.

## History

May 31th, 2021 - Initial version

## Copyright

All content herein is licensed under [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0).