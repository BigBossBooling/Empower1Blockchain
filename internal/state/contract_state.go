package state

import (
	"encoding/hex"
	"fmt"
	"sync"
)

// ContractStorageData represents the storage for a single contract.
// It's a map of key -> value (both []byte).
type ContractStorageData map[string][]byte

// GlobalContractStore holds the storage for all contracts.
// It's a map of contractAddress (hex string) -> ContractStorageData.
// This is a mock, in-memory implementation. A real system would use a persistent DB.
var GlobalContractStore = make(map[string]ContractStorageData)
var globalStoreLock = sync.RWMutex{}

// DeployedContractsStore holds the WASM bytecode for deployed contracts.
// map contractAddress (hex string) -> wasmCode []byte
var DeployedContractsStore = make(map[string][]byte)
var deployedContractsLock = sync.RWMutex{}


// --- Contract Code Storage ---

// StoreContractCode stores the WASM bytecode for a deployed contract.
func StoreContractCode(contractAddress []byte, code []byte) error {
	deployedContractsLock.Lock()
	defer deployedContractsLock.Unlock()

	addressHex := hex.EncodeToString(contractAddress)
	if _, exists := DeployedContractsStore[addressHex]; exists {
		return fmt.Errorf("contract code for address %s already exists", addressHex)
	}
	// Store a copy to prevent external modification
	codeCopy := make([]byte, len(code))
	copy(codeCopy, code)
	DeployedContractsStore[addressHex] = codeCopy
	// log.Printf("State: Stored code for contract %s (%d bytes)", addressHex, len(codeCopy))
	return nil
}

// GetContractCode retrieves the WASM bytecode for a given contract address.
func GetContractCode(contractAddress []byte) ([]byte, error) {
	deployedContractsLock.RLock()
	defer deployedContractsLock.RUnlock()

	addressHex := hex.EncodeToString(contractAddress)
	code, exists := DeployedContractsStore[addressHex]
	if !exists {
		return nil, fmt.Errorf("no contract code found for address %s", addressHex)
	}
	// Return a copy to prevent external modification
	codeCopy := make([]byte, len(code))
	copy(codeCopy, code)
	return codeCopy, nil
}


// --- Contract Data Storage ---

// GetContractStorage retrieves a value from a contract's storage.
// contractAddress: The address of the contract.
// key: The key within that contract's storage.
func GetContractStorage(contractAddress []byte, key []byte) ([]byte, error) {
	globalStoreLock.RLock()
	defer globalStoreLock.RUnlock()

	addressHex := hex.EncodeToString(contractAddress)
	keyHex := hex.EncodeToString(key)

	contractData, ok := GlobalContractStore[addressHex]
	if !ok {
		// log.Printf("State: GetContractStorage: No storage found for contract %s (key: %s)", addressHex, keyHex)
		return nil, nil // Key not found can be represented by nil value, nil error
	}

	value, ok := contractData[keyHex]
	if !ok {
		// log.Printf("State: GetContractStorage: Key %s not found in contract %s", keyHex, addressHex)
		return nil, nil // Key not found
	}

	// Return a copy to prevent external modification of the stored slice
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)
	// log.Printf("State: GetContractStorage: Contract %s, Key %s, Value %x (len %d)", addressHex, keyHex, valueCopy, len(valueCopy))
	return valueCopy, nil
}

// SetContractStorage sets a value in a contract's storage.
// contractAddress: The address of the contract.
// key: The key within that contract's storage.
// value: The value to set. If value is nil or empty, it can mean deleting the key.
func SetContractStorage(contractAddress []byte, key []byte, value []byte) error {
	globalStoreLock.Lock()
	defer globalStoreLock.Unlock()

	addressHex := hex.EncodeToString(contractAddress)
	keyHex := hex.EncodeToString(key)

	contractData, ok := GlobalContractStore[addressHex]
	if !ok {
		contractData = make(ContractStorageData)
		GlobalContractStore[addressHex] = contractData
	}

	if value == nil { // Standard practice: setting a key to nil deletes it
		// log.Printf("State: SetContractStorage: Deleting key %s for contract %s", keyHex, addressHex)
		delete(contractData, keyHex)
	} else {
		// Store a copy
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		contractData[keyHex] = valueCopy
		// log.Printf("State: SetContractStorage: Contract %s, Key %s, Value %x (len %d)", addressHex, keyHex, valueCopy, len(valueCopy))
	}
	return nil
}

// GenerateContractAddress (very basic): derives a contract address.
// A real implementation would be more robust, e.g., hash(deployerAddress + nonce).
// For now, just hash the code itself or use a counter for uniqueness in tests.
// This is a placeholder and likely needs to live elsewhere (e.g., in a core utility or when processing deployment tx).
var contractAddressCounter = 0 // Highly simplified, for testing only
var contractAddrMutex sync.Mutex

func GenerateNewContractAddress(deployerAddress []byte) []byte {
	contractAddrMutex.Lock()
	defer contractAddrMutex.Unlock()
	// Simple address generation for now: "contract" + counter
	// This is NOT how it should be done in production.
	contractAddressCounter++
	addrString := fmt.Sprintf("contract_addr_%d_from_%s", contractAddressCounter, hex.EncodeToString(deployerAddress[:min(4, len(deployerAddress))]))
	return []byte(addrString) // Using string as []byte for simplicity with current hex storage keys
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
