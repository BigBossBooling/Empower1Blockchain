package p2p

import (
	"bytes"
	"encoding/gob"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output (optional, as logger should be configured centrally)
	"time"   // Added for potential timestamps in future messages
	
	"empower1.com/core/core" // Assuming 'core' is the package alias for empower1.com/core/core
)

// --- Custom Errors for P2P Message Handling ---
var (
	ErrMessageSerialization   = errors.New("failed to serialize p2p message")
	ErrMessageDeserialization = errors.New("failed to deserialize p2p message")
	ErrPayloadEncoding        = errors.New("failed to encode p2p payload")
	ErrPayloadDecoding        = errors.New("failed to decode p2p payload")
	ErrUnknownMessageType     = errors.New("unknown message type received")
	ErrInvalidPayloadType     = errors.New("payload type mismatch for message")
)

// MessageType represents the type of a message in the P2P network.
// These constants define the communication protocols between nodes.
type MessageType byte

const (
	// --- Handshake & Peer Discovery ---
	MsgHello           MessageType = iota // Sent by a node when it first connects to a peer.
	MsgPeerList                           // Sent to share known peers.
	MsgRequestPeerList                    // Sent to request a list of known peers.

	// --- Consensus & Chain Sync ---
	MsgNewBlockProposal // Sent when a validator proposes a new block.
	MsgBlockVote        // Sent by validators to vote on a proposed block (rudimentary for now).
	MsgBlockRequest     // Sent to request a specific block by hash or height.
	MsgBlockResponse    // Response to MsgBlockRequest, containing the block.

	// --- Transaction Propagation ---
	MsgNewTransaction // Sent to broadcast a new transaction to the network.

	// --- EmPower1 Specific (Conceptual V2+) ---
	MsgAILog          // Sent to broadcast AI/ML audit logs or related data.
	MsgWealthUpdate   // Sent to signal significant wealth level updates for social mining (PoP).
)

// Message represents a generic message exchanged between peers.
// This is the fundamental unit of communication across the network.
type Message struct {
	Type      MessageType // Type of message, indicates expected Payload content
	Timestamp int64       // Message creation timestamp (for freshness checks, anti-replay)
	SenderID  []byte      // ID of the sender (e.g., node ID or public key hash)
	Payload   []byte      // Raw bytes of the gob-encoded specific payload struct
}

// --- Payload Structs ---
// These define the specific data structures carried by different message types.
// All payloads should be gob-encodable.

// HelloPayload is the payload for a MsgHello message.
type HelloPayload struct {
	Version      string   // Protocol version
	ListenAddr   string   // The address this node is listening on, if any (e.g., "127.0.0.1:8080")
	NodeID       []byte   // The cryptographic ID of the connecting node
	KnownPeers   []string // Network addresses of peers known by the sender
	CurrentHeight int64   // Sender's current blockchain height
}

// PeerListPayload is the payload for a MsgPeerList message.
type PeerListPayload struct {
	Peers []string // Network addresses of peers (e.g., "1.2.3.4:8080")
}

// NewBlockProposalPayload is the payload for a MsgNewBlockProposal.
type NewBlockProposalPayload struct {
	BlockData []byte // Gob-serialized core.Block
}

// BlockVotePayload is the payload for a MsgBlockVote. (Rudimentary PoS vote for V1)
type BlockVotePayload struct {
	BlockHash []byte // Hash of the block being voted on
	Validator []byte // Address of the voting validator
	Signature []byte // Signature of the vote message
	IsValid   bool   // True if the validator considers the block valid (for simple binary vote)
	// V2+: VoteRound int64 // For multi-round consensus protocols
}

// NewTransactionPayload is the payload for MsgNewTransaction.
type NewTransactionPayload struct {
	TransactionData []byte // Gob-serialized core.Transaction
}

// BlockRequestPayload is the payload for MsgBlockRequest.
type BlockRequestPayload struct {
	BlockHash []byte // Request block by its hash
	Height    int64  // Or request by height
}

// BlockResponsePayload is the payload for MsgBlockResponse.
type BlockResponsePayload struct {
	BlockData []byte // Gob-serialized core.Block
}

// --- Message Construction and Serialization/Deserialization ---

// NewMessage creates a new generic Message.
// This function helps in consistently creating messages before payload encoding.
func NewMessage(msgType MessageType, senderID []byte, payload []byte) *Message {
	return &Message{
		Type:      msgType,
		Timestamp: time.Now().UnixNano(),
		SenderID:  senderID,
		Payload:   payload,
	}
}

// Serialize serializes a Message struct into a byte slice using gob.
// This is the primary method for preparing messages for network transmission.
func (m *Message) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("%w: message type %s, sender %x: %v", ErrMessageSerialization, m.Type.String(), m.SenderID, err)
	}
	return buf.Bytes(), nil
}

// DeserializeMessage deserializes a byte slice into a Message using gob.
// This is the primary method for parsing incoming network messages.
func DeserializeMessage(data []byte) (*Message, error) {
	var msg Message
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&msg); err != nil {
		return nil, fmt.Errorf("%w: failed to gob decode message: %v", ErrMessageDeserialization, err)
	}
	if msg.SenderID == nil || len(msg.SenderID) == 0 { // Basic validation
		return nil, fmt.Errorf("%w: message missing sender ID", ErrMessageDeserialization)
	}
	return &msg, nil
}

// --- Payload Specific Serialization/Deserialization Helpers ---
// These functions provide type-safe encoding/decoding for message payloads,
// enforcing "Know Your Core, Keep it Clear" for data types.

// EncodePayload serializes any gob-encodable payload to []byte for inclusion in a Message.
func EncodePayload(payload interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(payload); err != nil {
		return nil, fmt.Errorf("%w: failed to gob encode payload of type %T: %v", ErrPayloadEncoding, payload, err)
	}
	return buf.Bytes(), nil
}

// DecodePayload deserializes raw payload bytes into a target payload struct.
func DecodePayload(data []byte, target interface{}) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("%w: failed to gob decode payload into type %T: %v", ErrPayloadDecoding, target, err)
	}
	return nil
}

// Helper specific deserializers for common payloads, for convenience and clarity.
func DeserializeHelloPayload(data []byte) (*HelloPayload, error) {
	var payload HelloPayload
	if err := DecodePayload(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode HelloPayload: %w", err)
	}
	return &payload, nil
}

func DeserializePeerListPayload(data []byte) (*PeerListPayload, error) {
	var payload PeerListPayload
	if err := DecodePayload(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode PeerListPayload: %w", err)
	}
	return &payload, nil
}

func DeserializeNewBlockProposalPayload(data []byte) (*NewBlockProposalPayload, error) {
	var payload NewBlockProposalPayload
	if err := DecodePayload(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode NewBlockProposalPayload: %w", err)
	}
	return &payload, nil
}

func DeserializeBlockVotePayload(data []byte) (*BlockVotePayload, error) {
	var payload BlockVotePayload
	if err := DecodePayload(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode BlockVotePayload: %w", err)
	}
	return &payload, nil
}

func DeserializeNewTransactionPayload(data []byte) (*NewTransactionPayload, error) {
	var payload NewTransactionPayload
	if err := DecodePayload(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode NewTransactionPayload: %w", err)
	}
	return &payload, nil
}

func DeserializeBlockRequestPayload(data []byte) (*BlockRequestPayload, error) {
	var payload BlockRequestPayload
	if err := DecodePayload(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode BlockRequestPayload: %w", err)
	}
	return &payload, nil
}

func DeserializeBlockResponsePayload(data []byte) (*BlockResponsePayload, error) {
	var payload BlockResponsePayload
	if err := DecodePayload(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode BlockResponsePayload: %w", err)
	}
	return &payload, nil
}

// String returns a human-readable string for MessageType.
// Useful for logging and debugging.
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
	case MsgBlockRequest:
		return "BLOCK_REQUEST"
	case MsgBlockResponse:
		return "BLOCK_RESPONSE"
	case MsgAILog: // EmPower1 specific
		return "AI_LOG"
	case MsgWealthUpdate: // EmPower1 specific
		return "WEALTH_UPDATE"
	default:
		return fmt.Sprintf("UNKNOWN_MSG_TYPE(%d)", mt)
	}
}

// GobRegisterTypes should be called once at application startup (e.g., in main.go init function).
// This is essential for gob to correctly encode/decode interface types or pointer types
// that are sent via Message.Payload. Without registration, gob might panic during deserialization.
func GobRegisterTypes() {
	gob.Register(&core.Block{})
	gob.Register(&core.Transaction{})
	gob.Register(&HelloPayload{})
	gob.Register(&PeerListPayload{})
	gob.Register(&NewBlockProposalPayload{})
	gob.Register(&BlockVotePayload{})
	gob.Register(&NewTransactionPayload{})
	gob.Register(&BlockRequestPayload{})
	gob.Register(&BlockResponsePayload{})
	// EmPower1 specific payload types (if any)
	// gob.Register(&AILogPayload{})
	// gob.Register(&WealthUpdatePayload{})
	log.Println("P2P: GOB types registered.")
}