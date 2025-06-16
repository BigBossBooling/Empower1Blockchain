package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64" // For new fields in TxDataForJSONHashing
	"encoding/gob"
	"encoding/json" // New import for JSON hashing
	"fmt"
	"sort" // For sorting AuthorizedPublicKeys for canonical JSON
	"time"
	"encoding/hex"
)

// TransactionType defines the type of transaction.
type TransactionType string

const (
	TxStandard           TransactionType = "standard"
	TxContractDeploy     TransactionType = "contract_deployment"
	TxContractCall       TransactionType = "contract_call"
	TxMultiSig           TransactionType = "multi_sig" // Potentially, a standard/call/deploy tx can *be* multi-sig
)

// SignerInfo holds a public key and its corresponding signature for multi-sig.
type SignerInfo struct {
	PublicKey []byte `json:"publicKey"` // Hex encoded for JSON, but []byte internally
	Signature []byte `json:"signature"` // Hex encoded for JSON, but []byte internally
}

// Transaction represents a standard transaction in the blockchain.
type Transaction struct {
	ID        []byte // Hash of the transaction content
	Timestamp int64

	// For single signer transactions OR as the multi-sig entity identifier
	From      []byte // Serialized Public Key of the sender OR MultiSig Address/Identifier

	// Single signer fields (used if TxType is not effectively multi-sig)
	PublicKey []byte // Sender's actual public key (if single signer)
	Signature []byte // Single signature

	TxType    TransactionType

	// Fields for TxStandard (can also be multi-sig)
	To     []byte
	Amount uint64
	Fee    uint64

	// Fields for TxContractDeploy (can also be multi-sig)
	ContractCode []byte

	// Fields for TxContractCall (can also be multi-sig)
	TargetContractAddress []byte
	FunctionName          string
	Arguments             []byte

	// Multi-Signature Fields
	// These are populated if this transaction is intended to be authorized by multiple signatures.
	// If these are present, 'From' should be the multi-sig address/identifier.
	// 'PublicKey' and 'Signature' fields above would be empty or ignored.
	RequiredSignatures  uint32       `json:"requiredSignatures,omitempty"` // M value
	AuthorizedPublicKeys [][]byte     `json:"authorizedPublicKeys,omitempty"` // N public keys (list of hex strings for JSON)
	Signers             []SignerInfo `json:"signers,omitempty"` // Collected M signatures
}


// TxDataForJSONHashing is used to create a canonical representation for hashing.
// For multi-sig, this payload (excluding Signers list) is what each signer signs.
// It must include all fields that define the transaction's intent.
// Order of fields matters for JSON canonical form if using struct tags for specific ordering,
// or rely on alphabetical sorting of map keys if converting to map[string]interface{} first.
// The Python side currently sorts keys alphabetically.
// This Go struct's fields MUST be in alphabetical order for JSON marshalling to match Python's output.
// The Signers field is EXCLUDED from this hashing structure, as it's what's being collected.
// However, the multi-sig configuration (M and N keys) IS part of what's signed.
type TxDataForJSONHashing struct {
	Amount                uint64   `json:"Amount,omitempty"`
	Arguments             string   `json:"Arguments,omitempty"`             // base64 of []byte
	AuthorizedPublicKeys  []string `json:"AuthorizedPublicKeys,omitempty"`  // List of hex strings
	ContractCode          string   `json:"ContractCode,omitempty"`          // base64 of []byte
	Fee                   uint64   `json:"Fee"`
	From                  string   `json:"From"`                          // hex string of Sender PubKey or MultiSig ID
	FunctionName          string   `json:"FunctionName,omitempty"`
	PublicKey             string   `json:"PublicKey,omitempty"`           // hex string (for single signer tx)
	RequiredSignatures    uint32   `json:"RequiredSignatures,omitempty"`  // M value
	TargetContractAddress string   `json:"TargetContractAddress,omitempty"` // hex string
	Timestamp             int64    `json:"Timestamp"`
	To                    string   `json:"To,omitempty"`                    // hex string
	TxType                string   `json:"TxType"`
}


// NewStandardTransaction creates a new standard value transfer transaction (single signer).
func NewStandardTransaction(from *ecdsa.PublicKey, to *ecdsa.PublicKey, amount uint64, fee uint64) (*Transaction, error) {
	if from == nil || to == nil {
		return nil, fmt.Errorf("sender and receiver public keys must be provided for standard transaction")
	}
	fromBytes := elliptic.Marshal(elliptic.P256(), from.X, from.Y)
	toBytes := elliptic.Marshal(elliptic.P256(), to.X, to.Y)

	tx := &Transaction{
		Timestamp:    time.Now().UnixNano(),
		From:         fromBytes,
		PublicKey:    fromBytes,
		TxType:       TxStandard,
		To:           toBytes,
		Amount:       amount,
		Fee:          fee,
	}
	return tx, nil
}

// NewContractDeploymentTransaction creates a new contract deployment transaction.
func NewContractDeploymentTransaction(deployer *ecdsa.PublicKey, contractCode []byte, fee uint64) (*Transaction, error) {
	if deployer == nil {
		return nil, fmt.Errorf("deployer public key must be provided")
	}
	if len(contractCode) == 0 {
		return nil, fmt.Errorf("contract code cannot be empty for deployment")
	}
	fromBytes := elliptic.Marshal(elliptic.P256(), deployer.X, deployer.Y)

	tx := &Transaction{
		Timestamp:    time.Now().UnixNano(),
		From:         fromBytes,
		PublicKey:    fromBytes, // For single signer deployment
		TxType:       TxContractDeploy,
		ContractCode: contractCode,
		Fee:          fee,
	}
	return tx, nil
}

// NewContractCallTransaction creates a new contract call transaction (single signer).
func NewContractCallTransaction(caller *ecdsa.PublicKey, contractAddress []byte, functionName string, args []byte, fee uint64) (*Transaction, error) {
	if caller == nil {
		return nil, fmt.Errorf("caller public key must be provided")
	}
	if len(contractAddress) == 0 {
		return nil, fmt.Errorf("target contract address must be provided")
	}
	if functionName == "" {
		return nil, fmt.Errorf("function name cannot be empty for contract call")
	}
	fromBytes := elliptic.Marshal(elliptic.P256(), caller.X, caller.Y)

	tx := &Transaction{
		Timestamp:             time.Now().UnixNano(),
		From:                  fromBytes,
		PublicKey:             fromBytes, // For single signer call
		TxType:                TxContractCall,
		TargetContractAddress: contractAddress,
		FunctionName:          functionName,
		Arguments:             args,
		Fee:                   fee,
	}
	return tx, nil
}


// Old NewTransaction - to be removed or refactored
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
	// The Python side uses these exact field names, capitalized, and json.dumps sorts them.
	// So, this Go struct must have its fields alphabetized for `json.Marshal` to match.
	// Fields are `omitempty` where appropriate.

	hashingStruct := TxDataForJSONHashing{
		Fee:       tx.Fee,
		From:      hex.EncodeToString(tx.From),
		Timestamp: tx.Timestamp,
		TxType:    string(tx.TxType),
	}

	// Single-signer PublicKey (empty for pure multi-sig where From is multi-sig ID)
	if len(tx.PublicKey) > 0 && tx.RequiredSignatures == 0 {
		hashingStruct.PublicKey = hex.EncodeToString(tx.PublicKey)
	}

	// Amount - can be present for standard, deploy (0), or call (value)
	// Using omitempty, so only include if non-zero or if TxStandard
	if tx.TxType == TxStandard || tx.Amount != 0 {
	    hashingStruct.Amount = tx.Amount
	}


	// Standard transaction fields
	if tx.TxType == TxStandard {
		if len(tx.To) > 0 {
			hashingStruct.To = hex.EncodeToString(tx.To)
		}
	}

	// Contract deployment fields
	if tx.TxType == TxContractDeploy {
		if len(tx.ContractCode) > 0 {
			hashingStruct.ContractCode = base64.StdEncoding.EncodeToString(tx.ContractCode)
		}
	}

	// Contract call fields
	if tx.TxType == TxContractCall {
		if len(tx.TargetContractAddress) > 0 {
			hashingStruct.TargetContractAddress = hex.EncodeToString(tx.TargetContractAddress)
		}
		if tx.FunctionName != "" { // omitempty in struct tag handles empty string
			hashingStruct.FunctionName = tx.FunctionName
		}
		if len(tx.Arguments) > 0 {
			hashingStruct.Arguments = base64.StdEncoding.EncodeToString(tx.Arguments)
		}
	}

	// Multi-signature configuration fields (part of what's signed)
	if tx.RequiredSignatures > 0 && len(tx.AuthorizedPublicKeys) > 0 {
		hashingStruct.RequiredSignatures = tx.RequiredSignatures

		hexKeys := make([]string, len(tx.AuthorizedPublicKeys))
		for i, pkBytes := range tx.AuthorizedPublicKeys {
			hexKeys[i] = hex.EncodeToString(pkBytes)
		}
		sort.Strings(hexKeys) // Sort the hex strings alphabetically for canonical JSON
		hashingStruct.AuthorizedPublicKeys = hexKeys
	}

	jsonBytes, err := json.Marshal(hashingStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction data to JSON for hashing: %w", err)
	}
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

// VerifySignature checks the transaction's signature(s).
// It handles both single-signer and multi-signer transactions.
func (tx *Transaction) VerifySignature() (bool, error) {
	// If multi-sig fields are populated, assume it's a multi-sig transaction.
	isMultiSig := tx.RequiredSignatures > 0 && len(tx.AuthorizedPublicKeys) > 0

	if isMultiSig {
		return tx.verifyMultiSignature()
	}
	return tx.verifySingleSignature()
}

// verifySingleSignature handles validation for non-multi-sig transactions.
func (tx *Transaction) verifySingleSignature() (bool, error) {
	if tx.PublicKey == nil || len(tx.PublicKey) == 0 {
		return false, fmt.Errorf("public key is missing for single-signer transaction")
	}
	if tx.Signature == nil || len(tx.Signature) == 0 {
		return false, fmt.Errorf("signature is missing for single-signer transaction")
	}

	// Deserialize the public key
	x, y := elliptic.Unmarshal(elliptic.P256(), tx.PublicKey)
	if x == nil {
		return false, fmt.Errorf("failed to unmarshal public key for single-signer")
	}
	publicKeyObj := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	txHash, err := tx.Hash() // This hash now includes multi-sig config if present, which is fine.
	if err != nil {
		return false, fmt.Errorf("failed to hash transaction for single-signer verification: %w", err)
	}

	valid := ecdsa.VerifyASN1(publicKeyObj, txHash, tx.Signature)
	return valid, nil
}

// verifyMultiSignature handles validation for multi-signature transactions.
func (tx *Transaction) verifyMultiSignature() (bool, error) {
	if tx.RequiredSignatures == 0 {
		return false, fmt.Errorf("RequiredSignatures (M) must be greater than 0 for multi-sig")
	}
	if uint32(len(tx.Signers)) < tx.RequiredSignatures {
		return false, fmt.Errorf("not enough signers: have %d, require %d", len(tx.Signers), tx.RequiredSignatures)
	}
	if len(tx.AuthorizedPublicKeys) == 0 {
		return false, fmt.Errorf("AuthorizedPublicKeys (N) must be provided for multi-sig")
	}
	if tx.RequiredSignatures > uint32(len(tx.AuthorizedPublicKeys)) {
		return false, fmt.Errorf("M (%d) cannot be greater than N (%d)", tx.RequiredSignatures, len(tx.AuthorizedPublicKeys))
	}

	// Calculate the hash of the transaction content (payload + multi-sig config)
	// This is what each signer should have signed.
	txHash, err := tx.Hash()
	if err != nil {
		return false, fmt.Errorf("failed to hash transaction for multi-sig verification: %w", err)
	}

	// Keep track of unique public keys that have provided valid signatures
	validSignerPubKeys := make(map[string]bool)

	for i, signerInfo := range tx.Signers {
		if signerInfo.PublicKey == nil || len(signerInfo.PublicKey) == 0 {
			return false, fmt.Errorf("public key missing for signer %d", i)
		}
		if signerInfo.Signature == nil || len(signerInfo.Signature) == 0 {
			return false, fmt.Errorf("signature missing for signer %d (pubkey: %x)", i, signerInfo.PublicKey)
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
			return false, fmt.Errorf("signer %d (pubkey: %x) is not in the authorized list", i, signerInfo.PublicKey)
		}

		// 2. Check for duplicate signers (by public key)
		signerPubKeyHex := hex.EncodeToString(signerInfo.PublicKey)
		if validSignerPubKeys[signerPubKeyHex] {
			return false, fmt.Errorf("duplicate signature from public key: %s", signerPubKeyHex)
		}

		// 3. Verify the signature
		x, y := elliptic.Unmarshal(elliptic.P256(), signerInfo.PublicKey)
		if x == nil {
			return false, fmt.Errorf("failed to unmarshal public key for signer %d (pubkey: %x)", i, signerInfo.PublicKey)
		}
		publicKeyObj := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

		if !ecdsa.VerifyASN1(publicKeyObj, txHash, signerInfo.Signature) {
			return false, fmt.Errorf("invalid signature for signer %d (pubkey: %x)", i, signerInfo.PublicKey)
		}

		validSignerPubKeys[signerPubKeyHex] = true
	}

	// 4. Ensure the number of unique valid signatures meets the M requirement
	if uint32(len(validSignerPubKeys)) < tx.RequiredSignatures {
		// This check is technically redundant if the outer loop iterates over tx.Signers
		// and len(tx.Signers) was already checked against tx.RequiredSignatures,
		// AND we ensure no duplicate signers. If tx.Signers could have more than M entries
		// (e.g. more than required people signed), then this final count is important.
		// Assuming tx.Signers will have at most N entries, and we need at least M valid ones.
		return false, fmt.Errorf("sufficient number of valid signatures not met: have %d, require %d", len(validSignerPubKeys), tx.RequiredSignatures)
	}

	// TODO: Verify the multi-sig identifier in tx.From matches the configuration
	// This requires deriving the address from tx.RequiredSignatures and tx.AuthorizedPublicKeys
	// and comparing it to tx.From. This function should be available from where address derivation is defined.
	// For now, skipping this direct check here, assuming it's done at a higher level or implicitly.

	return true, nil
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
