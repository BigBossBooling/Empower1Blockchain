package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic" // For P256 curve
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary" // Added for canonical byte representation of numbers
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"sort"   // For sorting AuthorizedPublicKeys

	// Removed unused "time" from here, but still used in functions where needed.
)

// --- Custom Error Definitions ---
// Define specific error types for clearer handling, crucial for financial integrity.
var (
	ErrInvalidTransaction       = errors.New("invalid transaction")
	ErrInvalidAddress           = errors.New("invalid address")
	ErrInsufficientFunds        = errors.New("insufficient funds") // Conceptual, for later UTXO/balance checks
	ErrSignatureMissingOrInvalid = errors.New("signature missing or invalid")
	ErrPublicKeyMissingOrInvalid = errors.New("public key missing or invalid")
	ErrContractCodeEmpty        = errors.New("contract code cannot be empty")
	ErrFunctionNameEmpty        = errors.New("function name cannot be empty")
	ErrTargetAddressMissing     = errors.New("target contract address missing")
	ErrMultiSigConfigInvalid    = errors.New("multi-signature configuration invalid")
	ErrNotEnoughSigners         = errors.New("not enough valid signatures provided")
	ErrUnauthorizedSigner       = errors.New("signer not in authorized public keys list")
	ErrDuplicateSigner          = errors.New("duplicate signature from same public key")
	ErrTransactionHashing       = errors.New("failed to hash transaction data")
	ErrTransactionSerialization = errors.New("failed to serialize transaction")
	ErrTransactionDeserialization = errors.New("failed to deserialize transaction")
)

// TransactionType defines the type of transaction.
type TransactionType string

const (
	TxStandard       TransactionType = "standard"
	TxContractDeploy TransactionType = "contract_deployment"
	TxContractCall   TransactionType = "contract_call"
	// TxMultiSig is not a type itself, but a property of other TxTypes.
	// Removed as a top-level TxType to reduce ambiguity.
)

// SignerInfo holds a public key and its corresponding signature for a multi-sig transaction.
// The `json` tags ensure proper serialization/deserialization.
type SignerInfo struct {
	PublicKey []byte `json:"publicKey"` // Raw bytes, will be hex-encoded for JSON output
	Signature []byte `json:"signature"` // Raw bytes, will be base64-encoded for JSON output
}

// Transaction represents a transaction in the blockchain.
// This struct is comprehensive, supporting standard transfers, contract deployments,
// contract calls, and multi-signature authorizations.
type Transaction struct {
	ID        []byte          `json:"id"`        // SHA256 hash of the canonical transaction payload
	Timestamp int64           `json:"timestamp"` // Unix nanoseconds
	TxType    TransactionType `json:"txType"`    // Type of transaction (e.g., "standard", "contract_deployment")

	// Standard Transaction Fields (TxStandard)
	From   []byte `json:"from,omitempty"`   // Sender's public key bytes (for single-sig) or MultiSig ID (for multi-sig)
	To     []byte `json:"to,omitempty"`     // Recipient's public key hash bytes
	Amount uint64 `json:"amount,omitempty"` // Value transferred

	// Contract Deployment Fields (TxContractDeploy)
	ContractCode []byte `json:"contractCode,omitempty"` // Compiled contract bytecode

	// Contract Call Fields (TxContractCall)
	TargetContractAddress []byte `json:"targetContractAddress,omitempty"` // Address of the contract to call
	FunctionName          string `json:"functionName,omitempty"`          // Name of the function to call
	Arguments             []byte `json:"arguments,omitempty"`             // Encoded arguments for the function call

	// Transaction Fee (common to most types)
	Fee uint64 `json:"fee"` // Fee paid for transaction processing

	// Single-Signature Fields
	// These are populated for standard single-signer transactions.
	// If multi-signature fields (RequiredSignatures, AuthorizedPublicKeys, Signers) are present,
	// these single-signature fields are typically ignored/empty (except 'From' which becomes MultiSig ID).
	PublicKey []byte `json:"publicKey,omitempty"` // Sender's actual public key bytes
	Signature []byte `json:"signature,omitempty"` // Single cryptographic signature

	// Multi-Signature Fields (M-of-N Multi-Sig)
	// These are populated if this transaction requires multiple authorizations.
	// 'From' should be the derived multi-sig address/identifier when these are used.
	RequiredSignatures   uint32       `json:"requiredSignatures,omitempty"`   // M: Minimum number of signatures required
	AuthorizedPublicKeys [][]byte     `json:"authorizedPublicKeys,omitempty"` // N: List of all authorized public keys (raw bytes)
	Signers              []SignerInfo `json:"signers,omitempty"`              // Collected individual signatures
}

// CanonicalTxPayload defines the structure for data that is hashed for transaction ID and signing.
// Fields are explicitly ordered and typed to ensure consistent JSON serialization
// matching Python's `json.dumps(..., sort_keys=True)`.
// This struct includes all fields that define the transaction's intent, but EXCLUDES collected signatures (Signers).
// Note: Fields are alphabetized based on their JSON tag names for canonical marshaling.
type CanonicalTxPayload struct {
	Amount                uint64   `json:"amount,omitempty"`
	Arguments             string   `json:"arguments,omitempty"` // base64 of []byte
	AuthorizedPublicKeys  []string `json:"authorizedPublicKeys,omitempty"` // List of hex strings
	ContractCode          string   `json:"contractCode,omitempty"`          // base64 of []byte
	Fee                   uint64   `json:"fee"`
	From                  string   `json:"from"` // hex string of Sender PubKey or MultiSig ID
	FunctionName          string   `json:"functionName,omitempty"`
	PublicKey             string   `json:"publicKey,omitempty"` // hex string (for single signer tx)
	RequiredSignatures    uint32   `json:"requiredSignatures,omitempty"` // M value
	TargetContractAddress string   `json:"targetContractAddress,omitempty"` // hex string
	Timestamp             int64    `json:"timestamp"`
	To                    string   `json:"to,omitempty"` // hex string
	TxType                string   `json:"txType"`
}

// --- Transaction Constructors (New Functionality) ---
// Adhere to "Know Your Core, Keep it Clear" by providing explicit constructors for different types.

// NewStandardTransaction creates a new single-signer value transfer transaction.
func NewStandardTransaction(fromPubKey *ecdsa.PublicKey, toPubKeyHash []byte, amount uint64, fee uint64) (*Transaction, error) {
    if fromPubKey == nil || len(toPubKeyHash) == 0 {
        return nil, ErrInvalidTransaction // Use specific error
    }
    senderPubKeyBytes := elliptic.Marshal(elliptic.P256(), fromPubKey.X, fromPubKey.Y)

    tx := &Transaction{
        Timestamp: time.Now().UnixNano(),
        From:      senderPubKeyBytes, // Sender's pubkey as 'From'
        PublicKey: senderPubKeyBytes, // Redundant for single-sig, but explicit for clarity
        TxType:    TxStandard,
        To:        toPubKeyHash,
        Amount:    amount,
        Fee:       fee,
    }
    // Hash and sign will be done separately (Sign method updates ID)
    return tx, nil
}

// NewContractDeploymentTransaction creates a new contract deployment transaction (single signer).
func NewContractDeploymentTransaction(deployerPubKey *ecdsa.PublicKey, contractCode []byte, fee uint64) (*Transaction, error) {
    if deployerPubKey == nil || len(contractCode) == 0 {
        return nil, ErrContractCodeEmpty // Use specific error
    }
    senderPubKeyBytes := elliptic.Marshal(elliptic.P256(), deployerPubKey.X, deployerPubKey.Y)

    tx := &Transaction{
        Timestamp:    time.Now().UnixNano(),
        From:         senderPubKeyBytes,
        PublicKey:    senderPubKeyBytes,
        TxType:       TxContractDeploy,
        ContractCode: contractCode,
        Fee:          fee,
    }
    return tx, nil
}

// NewContractCallTransaction creates a new contract call transaction (single signer).
func NewContractCallTransaction(callerPubKey *ecdsa.PublicKey, contractAddress []byte, functionName string, args []byte, fee uint64) (*Transaction, error) {
    if callerPubKey == nil || len(contractAddress) == 0 || functionName == "" {
        return nil, ErrInvalidTransaction // Consolidated check
    }
    senderPubKeyBytes := elliptic.Marshal(elliptic.P256(), callerPubKey.X, callerPubKey.Y)

    tx := &Transaction{
        Timestamp:             time.Now().UnixNano(),
        From:                  senderPubKeyBytes,
        PublicKey:             senderPubKeyBytes,
        TxType:                TxContractCall,
        TargetContractAddress: contractAddress,
        FunctionName:          functionName,
        Arguments:             args,
        Fee:                   fee,
    }
    return tx, nil
}

// NewMultiSigTransaction creates a transaction requiring M-of-N signatures.
// 'from' parameter should conceptually be the derived multi-sig address/identifier.
func NewMultiSigTransaction(
	fromMultiSigID []byte,
	txType TransactionType,
	requiredSignatures uint32,
	authorizedPublicKeys [][]byte,
	amount uint64, // Common for standard/call types that have amount
	toPubKeyHash []byte, // For standard
	contractCode []byte, // For deploy
	targetContractAddress []byte, // For call
	functionName string, // For call
	args []byte, // For call
	fee uint64,
) (*Transaction, error) {
	if len(fromMultiSigID) == 0 || requiredSignatures == 0 || len(authorizedPublicKeys) == 0 {
		return nil, ErrMultiSigConfigInvalid // Basic validation
	}
	if requiredSignatures > uint32(len(authorizedPublicKeys)) {
		return nil, ErrMultiSigConfigInvalid
	}

	tx := &Transaction{
		Timestamp:          time.Now().UnixNano(),
		From:               fromMultiSigID, // Multi-sig ID as 'From'
		TxType:             txType,
		Fee:                fee,
		RequiredSignatures: requiredSignatures,
		AuthorizedPublicKeys: authorizedPublicKeys,
		Signers:            []SignerInfo{}, // Initialize empty
	}

	// Populate type-specific fields
	switch txType {
	case TxStandard:
		tx.Amount = amount
		tx.To = toPubKeyHash
	case TxContractDeploy:
		tx.ContractCode = contractCode
	case TxContractCall:
		tx.TargetContractAddress = targetContractAddress
		tx.FunctionName = functionName
		tx.Arguments = args
		tx.Amount = amount // Call can also transfer value
	default:
		return nil, ErrInvalidTransaction // Unsupported type for multi-sig
	}

	return tx, nil
}


// --- Hashing and Signing Logic ---

// prepareDataForHashing creates a canonical JSON representation of the transaction's core data.
// This canonical form ensures consistent hashing across different environments and languages (e.g., Go and Python).
// It includes all fields that define the transaction's intent, but EXCLUDES collected signatures (Signers) and final ID.
func (tx *Transaction) prepareDataForHashing() ([]byte, error) {
    // Populate the canonical hashing struct.
    // Ensure all byte slices are hex-encoded for JSON representation.
    // Ensure `Arguments` and `ContractCode` are base64-encoded.
    hashingStruct := CanonicalTxPayload{
        Fee:       tx.Fee,
        From:      hex.EncodeToString(tx.From),
        Timestamp: tx.Timestamp,
        TxType:    string(tx.TxType),
    }

    if tx.Amount != 0 { // Omit if zero unless TxStandard specifically requires 0
		hashingStruct.Amount = tx.Amount
	} else if tx.TxType == TxStandard {
		hashingStruct.Amount = 0 // Explicitly set 0 for standard if needed
	}

    if len(tx.PublicKey) > 0 { // Only for single signer tx
        hashingStruct.PublicKey = hex.EncodeToString(tx.PublicKey)
    }

    if len(tx.To) > 0 {
        hashingStruct.To = hex.EncodeToString(tx.To)
    }

    if len(tx.ContractCode) > 0 {
        hashingStruct.ContractCode = base64.StdEncoding.EncodeToString(tx.ContractCode)
    }

    if len(tx.TargetContractAddress) > 0 {
        hashingStruct.TargetContractAddress = hex.EncodeToString(tx.TargetContractAddress)
    }
    if tx.FunctionName != "" { 
        hashingStruct.FunctionName = tx.FunctionName
    }
    if len(tx.Arguments) > 0 {
        hashingStruct.Arguments = base64.StdEncoding.EncodeToString(tx.Arguments)
    }

    // Multi-signature configuration fields (part of what's signed)
    if tx.RequiredSignatures > 0 && len(tx.AuthorizedPublicKeys) > 0 {
        hashingStruct.RequiredSignatures = tx.RequiredSignatures
        hexKeys := make([]string, len(tx.AuthorizedPublicKeys))
        for i, pkBytes := range tx.AuthorizedPublicKeys {
            hexKeys[i] = hex.EncodeToString(pkBytes)
        }
        sort.Strings(hexKeys) // Crucial for canonical JSON order
        hashingStruct.AuthorizedPublicKeys = hexKeys
    }

    // json.Marshal on a struct with explicit `json:"field,omitempty"` tags
    // and correctly ordered fields produces canonical JSON for hashing.
    jsonBytes, err := json.Marshal(hashingStruct)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to marshal transaction data to JSON for hashing: %v", ErrTransactionHashing, err)
    }
    return jsonBytes, nil
}

// Hash calculates the SHA256 hash of the transaction's canonical content.
// This hash serves as the transaction's unique ID.
func (tx *Transaction) Hash() ([]byte, error) {
    txDataBytes, err := tx.prepareDataForHashing()
    if err != nil {
        return nil, err
    }
    hash := sha256.Sum256(txDataBytes)
    return hash[:], nil
}

// Sign signs the transaction using the provided ECDSA private key.
// It sets the transaction's Signature and PublicKey fields (for single-sig) or adds to Signers (for multi-sig).
// The transaction ID should be set after successful signing.
func (tx *Transaction) Sign(privateKey *ecdsa.PrivateKey) error {
    if privateKey == nil {
        return ErrSignatureMissingOrInvalid // Use specific error
    }

    txHash, err := tx.Hash() // Hash of the canonical transaction payload
    if err != nil {
        return fmt.Errorf("%w: failed to hash transaction for signing: %v", ErrTransactionHashing, err)
    }

    sig, err := ecdsa.SignASN1(rand.Reader, privateKey, txHash)
    if err != nil {
        return fmt.Errorf("%w: failed to sign transaction: %v", ErrSignatureMissingOrInvalid, err)
    }
    
    signerPubKeyBytes := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)

    if tx.RequiredSignatures > 0 { // This is a multi-signature transaction
        // Add the signature to the Signers slice
        tx.Signers = append(tx.Signers, SignerInfo{
            PublicKey: signerPubKeyBytes,
            Signature: sig,
        })
    } else { // This is a single-signature transaction
        tx.PublicKey = signerPubKeyBytes
        tx.Signature = sig
        // For single-sig, 'From' is implicitly the Public Key of the signer
        tx.From = signerPubKeyBytes
    }

    // Set the transaction ID after signing to ensure it's finalized (often hash of content)
    tx.ID = txHash
    return nil
}

// VerifySignature checks the transaction's signature(s).
// It handles both single-signer and multi-signer transactions based on the presence of multi-sig fields.
func (tx *Transaction) VerifySignature() (bool, error) {
    isMultiSig := tx.RequiredSignatures > 0 || len(tx.AuthorizedPublicKeys) > 0 || len(tx.Signers) > 0

    if isMultiSig {
        return tx.verifyMultiSignature()
    }
    return tx.verifySingleSignature()
}

// verifySingleSignature handles validation for non-multi-sig transactions.
func (tx *Transaction) verifySingleSignature() (bool, error) {
    if len(tx.PublicKey) == 0 {
        return false, ErrPublicKeyMissingOrInvalid
    }
    if len(tx.Signature) == 0 {
        return false, ErrSignatureMissingOrInvalid
    }

    x, y := elliptic.Unmarshal(elliptic.P256(), tx.PublicKey)
    if x == nil || y == nil { // Ensure both X and Y are valid
        return false, ErrPublicKeyMissingOrInvalid
    }
    publicKeyObj := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

    txHash, err := tx.Hash() // Hash of the canonical transaction payload (same as tx.ID after signing)
    if err != nil {
        return false, fmt.Errorf("%w: failed to hash transaction for single-signer verification: %v", ErrTransactionHashing, err)
    }

    valid := ecdsa.VerifyASN1(publicKeyObj, txHash, tx.Signature)
    return valid, nil
}

// verifyMultiSignature handles validation for multi-signature transactions.
func (tx *Transaction) verifyMultiSignature() (bool, error) {
    // Basic multi-sig config validation
    if tx.RequiredSignatures == 0 || uint32(len(tx.AuthorizedPublicKeys)) == 0 {
        return false, ErrMultiSigConfigInvalid
    }
    if tx.RequiredSignatures > uint32(len(tx.AuthorizedPublicKeys)) {
        return false, fmt.Errorf("%w: M (%d) cannot be greater than N (%d)", ErrMultiSigConfigInvalid, tx.RequiredSignatures, len(tx.AuthorizedPublicKeys))
    }
    // Ensure enough signers are actually provided for the validation process
    if uint32(len(tx.Signers)) < tx.RequiredSignatures {
        return false, fmt.Errorf("%w: not enough signers provided for verification (have %d, require %d)", ErrNotEnoughSigners, len(tx.Signers), tx.RequiredSignatures)
    }


    txHash, err := tx.Hash() // This hash is of the transaction content (payload + multi-sig config)
    if err != nil {
        return false, fmt.Errorf("%w: failed to hash transaction for multi-sig verification: %v", ErrTransactionHashing, err)
    }

    validSignerPubKeys := make(map[string]bool) // Use map to track unique valid signers

    for i, signerInfo := range tx.Signers {
        if len(signerInfo.PublicKey) == 0 || len(signerInfo.Signature) == 0 {
            return false, fmt.Errorf("%w: signer %d has missing public key or signature", ErrSignatureMissingOrInvalid, i)
        }

        // 1. Check if signer's public key is one of the authorized public keys
        isAuthorized := false
        for _, authorizedKeyBytes := range tx.AuthorizedPublicKeys {
            if bytes.Equal(signerInfo.PublicKey, authorizedKeyBytes) {
                isAuthorized = true
                break
            }
        }
        if !isAuthorized {
            return false, fmt.Errorf("%w: signer %d (pubkey: %x) is not in the authorized list", ErrUnauthorizedSigner, i, signerInfo.PublicKey)
        }

        // 2. Check for duplicate signers (by public key) - only one valid signature per key
        signerPubKeyHex := hex.EncodeToString(signerInfo.PublicKey)
        if validSignerPubKeys[signerPubKeyHex] {
            return false, fmt.Errorf("%w: duplicate signature from public key: %s", ErrDuplicateSigner, signerPubKeyHex)
        }

        // 3. Verify the signature
        x, y := elliptic.Unmarshal(elliptic.P256(), signerInfo.PublicKey)
        if x == nil || y == nil {
            return false, fmt.Errorf("%w: failed to unmarshal public key for signer %d (pubkey: %x)", ErrPublicKeyMissingOrInvalid, i, signerInfo.PublicKey)
        }
        publicKeyObj := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

        if !ecdsa.VerifyASN1(publicKeyObj, txHash, signerInfo.Signature) {
            return false, fmt.Errorf("%w: invalid signature for signer %d (pubkey: %x)", ErrSignatureMissingOrInvalid, i, signerInfo.PublicKey)
        }

        validSignerPubKeys[signerPubKeyHex] = true // Mark this public key as having provided a valid signature
    }

    // 4. Final check: Ensure the number of unique valid signatures meets the M requirement
    if uint32(len(validSignerPubKeys)) < tx.RequiredSignatures {
        return false, fmt.Errorf("%w: sufficient number of unique valid signatures not met: have %d, require %d", ErrNotEnoughSigners, len(validSignerPubKeys), tx.RequiredSignatures)
    }

    // TODO: Verify the multi-sig identifier in tx.From matches the derived multi-sig address
    // This is a complex check that requires the multi-sig address derivation function,
    // which would typically reside in a separate address utility package.
    // This check is crucial for full multi-sig integrity but is deferred from this Tx verification scope.

    return true, nil
}


// --- Helper Functions (General Utilities) ---

// encodeInt64 converts an int64 to a byte slice using binary.BigEndian encoding.
// It panics only if an unexpected error occurs during encoding, which should not happen for int64.
func encodeInt64(num int64) []byte {
    buf := new(bytes.Buffer)
    err := binary.Write(buf, binary.BigEndian, num)
    if err != nil {
        panic(fmt.Sprintf("CORE_UTIL_PANIC: failed to encode int64: %v", err)) 
    }
    return buf.Bytes()
}

// --- Address and Key Utility Functions (Conceptual/Placeholder) ---
// These would typically live in a separate 'address' or 'crypto_util' package.
// Included here conceptually to show how parts interact for full understanding.

// GenerateKeyPairECDSA generates a new ECDSA private/public key pair (P256 curve).
func GenerateKeyPairECDSA() (*ecdsa.PrivateKey, error) {
    privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        return nil, fmt.Errorf("failed to generate ECDSA key pair: %w", err)
    }
    return privKey, nil
}

// PublicKeyToAddress derives a simplified address from an ECDSA public key.
// In a real blockchain, this would involve hashing the public key (e.g., SHA256 then RIPEMD160)
// and adding a version byte and checksum.
func PublicKeyToAddress(pubKey *ecdsa.PublicKey) []byte {
    pubKeyBytes := elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)
    // For simplicity, return the raw public key bytes as the address for now.
    // In production, this would be a hashed, checksummed address.
    return pubKeyBytes
}

// AddressFromPubKeyBytes (conceptual) reconstructs a PublicKey from address bytes (if address is raw pubkey)
func AddressFromPubKeyBytes(addrBytes []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), addrBytes)
	if x == nil || y == nil {
		return nil, ErrPublicKeyMissingOrInvalid
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// --- Serialization and Deserialization (GOB for internal, JSON for canonical hashing) ---

// Serialize serializes the entire Transaction struct using gob for efficient binary storage.
func (tx *Transaction) Serialize() ([]byte, error) {
    var buf bytes.Buffer
    encoder := gob.NewEncoder(&buf)
    if err := encoder.Encode(tx); err != nil {
        return nil, fmt.Errorf("%w: failed to serialize transaction: %v", ErrTransactionSerialization, err)
    }
    return buf.Bytes(), nil
}

// DeserializeTransaction deserializes bytes into a Transaction using gob.
func DeserializeTransaction(data []byte) (*Transaction, error) {
    var tx Transaction
    decoder := gob.NewDecoder(bytes.NewReader(data))
    if err := decoder.Decode(&tx); err != nil {
        return nil, fmt.Errorf("%w: failed to deserialize transaction: %v", ErrTransactionDeserialization, err)
    }
    // Re-calculate ID after deserialization to ensure integrity (optional, can be done at validation)
    // tx.ID, _ = tx.Hash()
    return &tx, nil
}