package p2p

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// MessageType represents the type of a message in the P2P network.
type MessageType byte

const (
	// MsgHello is sent by a node when it first connects to a peer.
	MsgHello MessageType = iota
	// MsgPeerList is sent to share known peers.
	MsgPeerList
	// MsgRequestPeerList is sent to request a list of known peers.
	MsgRequestPeerList
	// Consensus messages
	MsgNewBlockProposal // Sent when a validator proposes a new block
	MsgBlockVote        // Sent by validators to vote on a proposed block (rudimentary for now)
	MsgNewTransaction   // Sent to broadcast a new transaction to the network
	// Add other message types here as needed
)

// Message represents a generic message exchanged between peers.
type Message struct {
	Type    MessageType
	Payload []byte // Can be any gob-encodable structure, or raw bytes for simple messages
}

// --- Payload Structs ---

// HelloPayload is the payload for a MsgHello message.
type HelloPayload struct {
	Version    string   // Protocol version
	ListenAddr string   // The address this node is listening on, if any
	KnownPeers []string // Peers known by the sender (network addresses)
}

// PeerListPayload is the payload for a MsgPeerList message.
type PeerListPayload struct {
	Peers []string // Network addresses of peers
}

// NewBlockProposalPayload is the payload for a MsgNewBlockProposal.
type NewBlockProposalPayload struct {
	BlockData []byte // Serialized core.Block
}

// BlockVotePayload is the payload for a MsgBlockVote. (Rudimentary)
type BlockVotePayload struct {
	BlockHash []byte
	Validator string
	Signature []byte
	IsValid   bool
}

// NewTransactionPayload is the payload for MsgNewTransaction.
type NewTransactionPayload struct {
	TransactionData []byte // Serialized core.Transaction
}


// --- Serialization/Deserialization of Messages and Payloads ---

// Serialize serializes a Message struct into a byte slice using gob.
func (m *Message) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("failed to gob encode message: %w", err)
	}
	return buf.Bytes(), nil
}

// DeserializeMessage deserializes a byte slice into a Message using gob.
func DeserializeMessage(data []byte) (*Message, error) {
	var msg Message
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&msg); err != nil {
		return nil, fmt.Errorf("failed to gob decode message: %w", err)
	}
	return &msg, nil
}

// --- Payload Specific Serialization/Deserialization Helpers ---
// It's often good practice to have these for type safety and clarity,
// though for simple cases direct gob encoding/decoding of Message.Payload is also possible.

// ToBytes serializes any gob-encodable payload to []byte.
func ToBytes(payload interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(payload); err != nil {
		return nil, fmt.Errorf("failed to gob encode payload: %w", err)
	}
	return buf.Bytes(), nil
}

// DeserializeHelloPayload deserializes bytes to HelloPayload.
func DeserializeHelloPayload(data []byte) (*HelloPayload, error) {
	var payload HelloPayload
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to gob decode HelloPayload: %w", err)
	}
	return &payload, nil
}

// DeserializePeerListPayload deserializes bytes to PeerListPayload.
func DeserializePeerListPayload(data []byte) (*PeerListPayload, error) {
	var payload PeerListPayload
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to gob decode PeerListPayload: %w", err)
	}
	return &payload, nil
}

// DeserializeNewBlockProposalPayload deserializes bytes to NewBlockProposalPayload.
func DeserializeNewBlockProposalPayload(data []byte) (*NewBlockProposalPayload, error) {
	var payload NewBlockProposalPayload
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to gob decode NewBlockProposalPayload: %w", err)
	}
	return &payload, nil
}

// DeserializeBlockVotePayload deserializes bytes to BlockVotePayload.
func DeserializeBlockVotePayload(data []byte) (*BlockVotePayload, error) {
	var payload BlockVotePayload
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to gob decode BlockVotePayload: %w", err)
	}
	return &payload, nil
}

// DeserializeNewTransactionPayload deserializes bytes to NewTransactionPayload.
func DeserializeNewTransactionPayload(data []byte) (*NewTransactionPayload, error) {
	var payload NewTransactionPayload
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to gob decode NewTransactionPayload: %w", err)
	}
	return &payload, nil
}


// String returns a human-readable string for MessageType.
func (mt MessageType) String() string {
	switch mt {
	case MsgHello:
		return "HELLO"
	case MsgPeerList:
		return "PEER_LIST"
	case MsgRequestPeerList:
		return "REQUEST_PEER_LIST"
	case MsgNewBlockProposal:
		return "NEW_BLOCK_PROPOSAL"
	case MsgBlockVote:
		return "BLOCK_VOTE"
	case MsgNewTransaction:
		return "NEW_TRANSACTION"
	default:
		return fmt.Sprintf("UNKNOWN_MSG_TYPE(%d)", mt)
	}
}

// GobRegisterTypes should be called in an init() function in the main package
// (or any package that uses these types with gob encoding where ambiguity might occur,
// especially if interfaces are used for payloads).
// For now, we handle specific typed payloads, reducing direct need for gob.Register
// unless interfaces are used as the direct type for Message.Payload in gob.Encode/Decode.
// func GobRegisterTypes() {
//    gob.Register(&core.Block{}) // If core.Block is ever sent directly
//    gob.Register(&core.Transaction{}) // If core.Transaction is ever sent directly
//    gob.Register(HelloPayload{})
//    gob.Register(PeerListPayload{})
//    gob.Register(NewBlockProposalPayload{})
//    gob.Register(BlockVotePayload{})
//    gob.Register(NewTransactionPayload{})
// }
