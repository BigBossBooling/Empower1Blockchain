package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json" // New import for JSON hashing
	"fmt"
	// "sort" // Removed unused import
	"time"
	"encoding/hex"
)

// Transaction represents a standard transaction in the blockchain.
type Transaction struct {
	ID        []byte // Hash of the transaction content (excluding ID and Signature itself)
	Timestamp int64
	// SenderAddress string   // Derived from PublicKey for user-friendliness if needed
	// ReceiverAddress string // User-friendly address
	From      []byte   // Serialized Public Key of the sender
	To        []byte   // Serialized Public Key of the receiver
	Amount    uint64
	Fee       uint64   // Placeholder for now
	Signature []byte   // Digital signature of tx content (hash of Sender, Receiver, Amount, Fee, Nonce, Timestamp)
	PublicKey []byte   // Sender's public key, to aid signature verification (same as From)
	// Nonce  uint64 // To prevent replay attacks, if needed (can be part of signed data)
}

// TxDataForHashing is a temporary struct to prepare transaction data for hashing and signing.
// It excludes ID and Signature.
type TxDataForHashing struct {
	Timestamp int64
	From      []byte
	To        []byte
	Amount    uint64
	Fee       uint64
	PublicKey []byte // Redundant with From but explicitly part of signed data
}

// NewTransaction creates a new transaction.
// Signature is applied separately. ID is calculated after signing.
func NewTransaction(from *ecdsa.PublicKey, to *ecdsa.PublicKey, amount uint64, fee uint64) (*Transaction, error) {
	if from == nil || to == nil {
		return nil, fmt.Errorf("sender and receiver public keys must be provided")
	}

	fromBytes := elliptic.Marshal(elliptic.P256(), from.X, from.Y)
	toBytes := elliptic.Marshal(elliptic.P256(), to.X, to.Y)

	tx := &Transaction{
		Timestamp: time.Now().UnixNano(),
		From:      fromBytes,
		To:        toBytes,
		Amount:    amount,
		Fee:       fee,
		PublicKey: fromBytes, // Store the sender's public key
	}
	return tx, nil
}

// prepareDataForHashing creates a consistent byte representation of the transaction for hashing.
// It now uses a canonical JSON representation to ensure cross-language compatibility for hashing.
func (tx *Transaction) prepareDataForHashing() ([]byte, error) {
	// The fields included and their types must match what the Python wallet's
	// transaction.data_for_hashing() produces for its JSON string.
	// Python uses hex strings for From, To, PublicKey in its JSON.
	//
	// Removed unused 'data' map variable:
	// data := map[string]interface{}{ ... }

	// To achieve a canonical JSON form, we should sort the keys.
	// The Python side uses json.dumps with sort_keys=True.
	// Go's json.Marshal on a map does not guarantee order.
	// A common approach for canonical JSON:
	// 1. Marshal to map[string]interface{}
	// 2. Get keys, sort them.
	// 3. Construct JSON string by iterating in key order.
	// However, Go's json.Marshal on a struct *does* respect struct field order (mostly for simple cases).
	// A simpler approach that matches Python's `json.dumps(payload_to_hash, sort_keys=True, separators=(',', ':'))`
	// is to marshal the map and rely on the default behavior or ensure Python matches Go's default map marshaling
	// if it were stable (which it isn't for maps).
	//
	// Python's `json.dumps` with `sort_keys=True` is the target.
	// Go doesn't have a direct equivalent for sorting map keys before JSON marshaling in one step.
	// We must build the JSON string carefully or use a struct with ordered fields.
	//
	// Let's use a struct for deterministic JSON field order matching Python's expectation for hashing.
	// This struct will be used *only* for creating the JSON to be hashed.
	// The Python side uses these exact field names, capitalized.
	type TxDataForJSONHashing struct {
		Amount    uint64 `json:"Amount"`
		Fee       uint64 `json:"Fee"`
		From      string `json:"From"` // hex string
		PublicKey string `json:"PublicKey"` // hex string
		Timestamp int64  `json:"Timestamp"`
		To        string `json:"To"` // hex string
	}

	jsonHashingStruct := TxDataForJSONHashing{
		Amount:    tx.Amount,
		Fee:       tx.Fee,
		From:      hex.EncodeToString(tx.From),
		PublicKey: hex.EncodeToString(tx.PublicKey),
		Timestamp: tx.Timestamp,
		To:        hex.EncodeToString(tx.To),
	}

	// Marshal this struct. The field order is defined by the struct.
	// Python's sort_keys=True will produce: Amount,Fee,From,PublicKey,Timestamp,To (alphabetical)
	// So, the Go struct TxDataForJSONHashing fields must be in alphabetical order.
	jsonBytes, err := json.Marshal(jsonHashingStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction data to JSON for hashing: %w", err)
	}

	// Python's `separators=(',', ':')` produces compact JSON. Go's default is also compact.
	return jsonBytes, nil
}

// Hash calculates the SHA256 hash of the transaction's content (for signing and ID).
func (tx *Transaction) Hash() ([]byte, error) {
	txDataBytes, err := tx.prepareDataForHashing()
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(txDataBytes)
	return hash[:], nil
}

// Sign signs the transaction using the provided ECDSA private key.
// It sets the transaction's Signature and PublicKey fields.
// The transaction ID should be set after signing by hashing the signed transaction (or its hash).
func (tx *Transaction) Sign(privateKey *ecdsa.PrivateKey) error {
	if privateKey == nil {
		return fmt.Errorf("private key is required to sign transaction")
	}

	// Ensure PublicKey matches the private key
	pubKeyBytes := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)
	tx.PublicKey = pubKeyBytes
	tx.From = pubKeyBytes // Sender is derived from the private key used for signing

	txHash, err := tx.Hash() // This hash is of the transaction data *before* signature
	if err != nil {
		return fmt.Errorf("failed to hash transaction for signing: %w", err)
	}

	sig, err := ecdsa.SignASN1(rand.Reader, privateKey, txHash)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}
	tx.Signature = sig

	// After signing, the ID can be set. The ID is the hash of the signed transaction data's hash.
	// Or, more commonly, ID is simply the txHash itself (hash of content before signature).
	// Let's use txHash (hash of content) as the ID.
	tx.ID = txHash
	return nil
}

// VerifySignature verifies the transaction's signature against its content and T.PublicKey.
func (tx *Transaction) VerifySignature() (bool, error) {
	if tx.PublicKey == nil || len(tx.PublicKey) == 0 {
		return false, fmt.Errorf("public key is missing from transaction")
	}
	if tx.Signature == nil || len(tx.Signature) == 0 {
		return false, fmt.Errorf("signature is missing from transaction")
	}

	// Deserialize the public key
	x, y := elliptic.Unmarshal(elliptic.P256(), tx.PublicKey)
	if x == nil { // Check if unmarshal was successful
		return false, fmt.Errorf("failed to unmarshal public key")
	}
	publicKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	// Calculate the hash of the transaction content (the data that was actually signed)
	txHash, err := tx.Hash()
	if err != nil {
		return false, fmt.Errorf("failed to hash transaction for verification: %w", err)
	}

	// Verify the signature
	valid := ecdsa.VerifyASN1(publicKey, txHash, tx.Signature)
	return valid, nil
}

// Helper to get string representation of addresses (optional, for logging/display)
func (tx *Transaction) FromAddressString() string {
	return hex.EncodeToString(tx.From)
}

func (tx *Transaction) ToAddressString() string {
	return hex.EncodeToString(tx.To)
}

// Serialize serializes the transaction using gob.
func (tx *Transaction) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(tx); err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}
	return buf.Bytes(), nil
}

// DeserializeTransaction deserializes bytes into a Transaction using gob.
func DeserializeTransaction(data []byte) (*Transaction, error) {
	var tx Transaction
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&tx); err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %w", err)
	}
	return &tx, nil
}
