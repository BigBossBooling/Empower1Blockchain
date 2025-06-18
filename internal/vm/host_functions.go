package vm

import (
	"encoding/hex" // Added for logging addresses/keys
	"fmt"
	"log"
	"os"
	"time" // For mock timestamp in stub

	"github.com/wasmerio/wasmer-go/wasmer"

	// For Transaction types, Block (if needed)
	"empower1.com/core/crypto" // For cryptographic utilities like GenerateDIDKey, PublicKeyBytesToAddress
	"empower1.com/core/state"  // For state.State interaction
)

// --- Host Function Environment ---
// This struct provides the necessary context and access to blockchain services
// that WASM smart contracts can call.
type HostFunctionEnvironment struct {
	Memory          *wasmer.Memory   // WASM instance memory, allowing Go to read/write from/to WASM
	ContractAddress []byte           // The address of the smart contract currently being executed
	CallerPublicKey []byte           // The raw public key bytes of the caller of the current transaction
	GasTank         *GasTank         // Pointer to the GasTank for gas metering
	Instance        *wasmer.Instance // Reference to the WASM instance (for internal calls, V2+)
	Logger          *log.Logger      // Logger dedicated for host function output
	State           *state.State     // Reference to the global blockchain State for storage operations
	// V2+: BlockContext *core.Block // Current block details for contract context
	// V2+: TxContext    *core.Transaction // Current transaction details for contract context
}

// NewHostFunctionEnvironment creates a new HostFunctionEnvironment for a specific contract execution.
// It performs basic validation to ensure the environment is correctly initialized.
func NewHostFunctionEnvironment(contractAddr, callerPubKey []byte, gasTank *GasTank, state *state.State) (*HostFunctionEnvironment, error) {
	if len(contractAddr) == 0 {
		return nil, fmt.Errorf("%w: contract address cannot be empty", ErrInvalidHostEnv)
	}
	if len(callerPubKey) == 0 {
		return nil, fmt.Errorf("%w: caller public key cannot be empty", ErrInvalidHostEnv)
	}
	if gasTank == nil {
		return nil, fmt.Errorf("%w: gas tank cannot be nil", ErrInvalidHostEnv)
	}
	if state == nil {
		return nil, fmt.Errorf("%w: state manager cannot be nil", ErrInvalidHostEnv)
	}

	logger := log.New(os.Stdout, "HOST_FN: ", log.Ldate|log.Ltime|log.Lshortfile) // Specific logger for host functions
	return &HostFunctionEnvironment{
		ContractAddress: contractAddr,
		CallerPublicKey: callerPubKey,
		GasTank:         gasTank,
		Logger:          logger,
		State:           state,
	}, nil
}

// OnInstantiated is part of wasmer.WasmerEnv interface.
// It's called by Wasmer after the module is instantiated, allowing us to link memory and the instance itself.
func (env *HostFunctionEnvironment) OnInstantiated(instance *wasmer.Instance) error {
	memory, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return fmt.Errorf("%w: failed to get exported WASM memory during instantiation: %v", ErrWASMExportMissing, err)
	}
	env.Memory = memory
	env.Instance = instance // Store instance for potential inter-contract calls in V2+
	return nil
}

// --- Host Function Implementations ---
// These functions are the "system calls" that a smart contract can make to interact with the blockchain state
// or environment. Their signatures must precisely match the WASM function types defined in vm.go.
// Gas consumption is a primary concern for every host function.

// BlockchainLogMessage (env.host_log_message) - For contracts to log messages.
// Parameters: message_ptr (i32), message_len (i32)
// Results: none
func BlockchainLogMessage(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil {
		return nil, fmt.Errorf("%w: host memory not available for logging", ErrInvalidHostEnv)
	}

	const gasCostLogMessageBase = 10 // Base cost for any log call
	if err := hostEnv.GasTank.ConsumeGas(gasCostLogMessageBase); err != nil {
		return []wasmer.Value{}, err // Propagate ErrOutOfGas
	}

	messagePtr := args[0].I32()
	messageLen := args[1].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	// Validate memory access: essential for security and integrity
	if messagePtr < 0 || messageLen < 0 || int64(messagePtr)+int64(messageLen) > int64(len(wasmMemoryData)) {
		hostEnv.Logger.Errorf("VM_HOST_ERR: BlockchainLogMessage: invalid memory access ptr=%d, len=%d, mem_size=%d for contract %x", messagePtr, messageLen, len(wasmMemoryData), hostEnv.ContractAddress)
		return nil, fmt.Errorf("%w: memory access out of bounds in host_log_message", ErrCodeInvalidMemoryAccess)
	}

	gasCostPerByte := uint64(1) // Example: 1 gas per byte of log message
	if err := hostEnv.GasTank.ConsumeGas(uint64(messageLen) * gasCostPerByte); err != nil {
		return []wasmer.Value{}, err // Propagate ErrOutOfGas
	}

	message := string(wasmMemoryData[messagePtr : messagePtr+messageLen])
	hostEnv.Logger.Printf("CONTRACT_LOG (Addr: %x): %s", hostEnv.ContractAddress, message) // Log with contract context

	return []wasmer.Value{}, nil
}

// BlockchainSetStorage (env.blockchain_set_storage) - For contracts to write to their persistent storage.
// Parameters: key_ptr (i32), key_len (i32), value_ptr (i32), value_len (i32)
// Results: error_code (i32) -> ErrCodeSuccess (0) for success, others for failure
func BlockchainSetStorage(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil || hostEnv.State == nil { // State manager is now directly used
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: host memory or state not available for storage operations", ErrInvalidHostEnv)
	}
	if hostEnv.ContractAddress == nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: contract address not available for storage operations", ErrInvalidHostEnv)
	}

	const gasCostSetStorageBase = 100 // Base cost for any storage write
	if err := hostEnv.GasTank.ConsumeGas(gasCostSetStorageBase); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	keyPtr, keyLen := args[0].I32(), args[1].I32()
	valPtr, valLen := args[2].I32(), args[3].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	// Validate memory access for key
	if keyPtr < 0 || keyLen < 0 || int64(keyPtr)+int64(keyLen) > int64(len(wasmMemoryData)) {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("%w: key memory access out of bounds in host_set_storage", ErrCodeInvalidMemoryAccess)
	}
	key := make([]byte, keyLen)
	copy(key, wasmMemoryData[keyPtr:keyPtr+keyLen])

	// Validate memory access for value
	if valPtr < 0 || valLen < 0 || int64(valPtr)+int64(valLen) > int64(len(wasmMemoryData)) {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("%w: value memory access out of bounds in host_set_storage", ErrCodeInvalidMemoryAccess)
	}
	value := make([]byte, valLen)
	copy(value, wasmMemoryData[valPtr:valPtr+valLen])

	gasCostStorageBytes := uint64(keyLen + valLen) // Gas for key+value bytes stored
	if err := hostEnv.GasTank.ConsumeGas(gasCostStorageBytes); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	// Call the State Manager to persist the data.
	err := hostEnv.State.SetContractStorage(hostEnv.ContractAddress, key, value)
	if err != nil {
		hostEnv.Logger.Printf("VM_HOST_ERR: host_set_storage failed for contract %x, key %x: %v", hostEnv.ContractAddress, key, err)
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: state interaction failed in host_set_storage: %v", ErrHostFunctionExecution, err)
	}
	return []wasmer.Value{wasmer.NewI32(int32(ErrCodeSuccess))}, nil
}

// BlockchainGetStorage (env.blockchain_get_storage) - For contracts to read from their persistent storage.
// Parameters: key_ptr (i32), key_len (i32), ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_value_len (i32) -> actual length of value, 0 if not found, -ve for error codes
func BlockchainGetStorage(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil || hostEnv.State == nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: host memory or state not available for storage operations", ErrInvalidHostEnv)
	}
	if hostEnv.ContractAddress == nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: contract address not available for storage operations", ErrInvalidHostEnv)
	}

	const gasCostGetStorageBase = 50 // Base cost for any storage read
	if err := hostEnv.GasTank.ConsumeGas(gasCostGetStorageBase); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	keyPtr, keyLen := args[0].I32(), args[1].I32()
	retBufPtr, retBufLen := args[2].I32(), args[3].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	// Validate memory access for key
	if keyPtr < 0 || keyLen < 0 || int64(keyPtr)+int64(keyLen) > int64(len(wasmMemoryData)) {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("%w: key memory access out of bounds in host_get_storage", ErrCodeInvalidMemoryAccess)
	}
	key := make([]byte, keyLen)
	copy(key, wasmMemoryData[keyPtr:keyPtr+keyLen])

	gasCostKeyBytes := uint64(keyLen) // Gas for key bytes read
	if err := hostEnv.GasTank.ConsumeGas(gasCostKeyBytes); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	value, err := hostEnv.State.GetContractStorage(hostEnv.ContractAddress, key)
	if err != nil {
		hostEnv.Logger.Printf("VM_HOST_ERR: host_get_storage state error for contract %x, key %x: %v", hostEnv.ContractAddress, key, err)
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: state interaction failed in host_get_storage: %v", ErrHostFunctionExecution, err)
	}

	if value == nil { // Key not found in storage
		return []wasmer.Value{wasmer.NewI32(0)}, nil // Return 0 for actual_value_len
	}

	actualValueLen := int32(len(value))

	gasCostValueBytes := uint64(actualValueLen) // Gas for the size of data retrieved
	if err := hostEnv.GasTank.ConsumeGas(gasCostValueBytes); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	// Validate buffer for writing, even if only copying a part (for security)
	bytesToCopy := actualValueLen
	if actualValueLen > retBufLen {
		bytesToCopy = retBufLen // Only copy what fits, truncate
		hostEnv.Logger.Printf("VM_HOST_WARN: host_get_storage: buffer too small for key %x contract %x. Need %d, have %d. Value will be truncated.\n", key, hostEnv.ContractAddress, actualValueLen, retBufLen)
		// Return actual_value_len; contract will then know buffer was too small.
	}

	if bytesToCopy > 0 { // Only attempt copy if there's something to copy
		if retBufPtr < 0 || int64(retBufPtr)+int64(bytesToCopy) > int64(len(wasmMemoryData)) {
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("%w: return buffer memory access out of bounds in host_get_storage", ErrCodeInvalidMemoryAccess)
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], value[:bytesToCopy])
	}

	return []wasmer.Value{wasmer.NewI32(actualValueLen)}, nil // Return the *actual* total length of the value.
}

// BlockchainGetBalance (env.blockchain_get_balance) - For contracts to query an address's balance.
// Parameters: address_ptr (i32), address_len (i32)
// Results: balance (i64)
func BlockchainGetBalance(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil || hostEnv.State == nil {
		return []wasmer.Value{wasmer.NewI64(0)}, fmt.Errorf("%w: host memory or state not available for balance query", ErrInvalidHostEnv)
	}

	const gasCost = 100 // Cost for balance lookup
	if err := hostEnv.GasTank.ConsumeGas(gasCost); err != nil {
		return []wasmer.Value{wasmer.NewI64(0)}, err // Propagate ErrOutOfGas
	}

	addrPtr, addrLen := args[0].I32(), args[1].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if addrPtr < 0 || addrLen <= 0 || int64(addrPtr)+int64(addrLen) > int64(len(wasmMemoryData)) {
		hostEnv.Logger.Errorf("VM_HOST_ERR: BlockchainGetBalance: invalid memory access ptr=%d, len=%d", addrPtr, addrLen)
		return []wasmer.Value{wasmer.NewI64(0)}, fmt.Errorf("%w: address memory access out of bounds in get_balance", ErrCodeInvalidMemoryAccess)
	}
	addressBytes := wasmMemoryData[addrPtr : addrPtr+addrLen]

	balance, err := hostEnv.State.GetBalance(addressBytes)
	if err != nil {
		// Log error, but return 0 balance to contract if not found or other non-critical error
		hostEnv.Logger.Printf("VM_HOST_WARN: blockchain_get_balance for %x failed: %v", addressBytes, err)
		return []wasmer.Value{wasmer.NewI64(0)}, nil // Contract might interpret 0 as not found
	}

	return []wasmer.Value{wasmer.NewI64(int64(balance))}, nil
}

// BlockchainSendFunds (env.blockchain_send_funds) - For contracts to send funds.
// Parameters: to_addr_ptr (i32), to_addr_len (i32), amount (i64)
// Results: error_code (i32) -> ErrCodeSuccess (0) for success
func BlockchainSendFunds(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil || hostEnv.State == nil || hostEnv.Instance == nil { // Instance needed for internal call context
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: host environment incomplete for send_funds", ErrInvalidHostEnv)
	}

	const gasCost = 500 // Higher cost for state-changing operation
	if err := hostEnv.GasTank.ConsumeGas(gasCost); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	toAddrPtr, toAddrLen := args[0].I32(), args[1].I32()
	amount := args[2].I64()
	wasmMemoryData := hostEnv.Memory.Data()

	// Validate memory access for recipient address
	if toAddrPtr < 0 || toAddrLen <= 0 || int64(toAddrPtr)+int64(toAddrLen) > int64(len(wasmMemoryData)) {
		hostEnv.Logger.Errorf("VM_HOST_ERR: BlockchainSendFunds: invalid memory access for recipient address.")
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, nil
	}
	toAddress := wasmMemoryData[toAddrPtr : toAddrPtr+toAddrLen]

	// Basic validation: amount must be positive
	if amount <= 0 {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeBadArgument))}, nil
	}

	// TODO: Perform actual transfer logic via State manager (requires UTXO consumption/creation)
	// This would involve:
	// 1. Checking contract's own balance/UTXOs (contractAddress is sender).
	// 2. Finding spendable outputs from the contract's address.
	// 3. Creating a new internal transaction (Signed by contract's "internal" private key or implicit).
	// 4. Updating state manager by consuming contract's UTXOs and creating new ones for recipient.
	// This is a complex operation and requires access to the full blockchain context.
	// For now, this is a conceptual placeholder.

	hostEnv.Logger.Printf("CONTRACT_TRANSFER (Addr: %x): Contract wants to send %d to %x\n", hostEnv.ContractAddress, amount, toAddress)
	// Return success code only if transfer logic fully implemented and passes.
	return []wasmer.Value{wasmer.NewI32(int32(ErrCodeSuccess))}, nil
}

// BlockchainGetBlockTimestamp (env.blockchain_get_block_timestamp) - For contracts to query the current block timestamp.
// Parameters: none
// Results: timestamp (i64) in Unix nanoseconds.
func BlockchainGetBlockTimestamp(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if err := hostEnv.GasTank.ConsumeGas(5); err != nil {
		return []wasmer.Value{wasmer.NewI64(0)}, err
	}
	// TODO: Get actual block timestamp from hostEnv.BlockContext.Timestamp (if CurrentBlock is added)
	// For now, return a dummy timestamp.
	currentTime := time.Now().UnixNano()
	hostEnv.Logger.Debugf("VM_HOST: blockchain_get_block_timestamp: returning dummy timestamp %d", currentTime)
	return []wasmer.Value{wasmer.NewI64(currentTime)}, nil
}

// blockchain_get_caller_address (env.blockchain_get_caller_address) - For contracts to get the caller's address.
// Parameters: ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_len (i32) of the caller address (hex string of public key)
func BlockchainGetCallerAddress(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: host memory not available", ErrInvalidHostEnv)
	}

	const gasCostGetCaller = 20
	if err := hostEnv.GasTank.ConsumeGas(gasCostGetCaller); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	if hostEnv.CallerPublicKey == nil || len(hostEnv.CallerPublicKey) == 0 {
		hostEnv.Logger.Warnf("VM_HOST_WARN: BlockchainGetCallerAddress: CallerPublicKey not set in host environment. Returning error code.")
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodePublicKeyNotAvailable))}, nil // Return error code to WASM
	}

	// Address is the hex string of the uncompressed public key.
	callerAddressHex := hex.EncodeToString(hostEnv.CallerPublicKey)
	callerAddressBytes := []byte(callerAddressHex)
	actualLen := int32(len(callerAddressBytes))

	retBufPtr := args[0].I32()
	retBufLen := args[1].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	// Validate buffer size and copy
	bytesToCopy := min(actualLen, retBufLen)
	if bytesToCopy > 0 {
		if retBufPtr < 0 || int64(retBufPtr)+int64(bytesToCopy) > int64(len(wasmMemoryData)) {
			hostEnv.Logger.Errorf("VM_HOST_ERR: get_caller_address: return buffer memory access out of bounds for contract %x", hostEnv.ContractAddress)
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, nil
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], callerAddressBytes[:bytesToCopy])
	}

	hostEnv.Logger.Debugf("VM_HOST: blockchain_get_caller_address: returning address %s (len %d)", callerAddressHex, actualLen)
	return []wasmer.Value{wasmer.NewI32(actualLen)}, nil // Return actual length of the address string
}

// BlockchainGetCallerPublicKey (env.blockchain_get_caller_public_key) - For contracts to get the caller's raw public key bytes.
// Parameters: ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_len (i32) of the raw public key bytes (65 for P256 uncompressed)
func BlockchainGetCallerPublicKey(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: host memory not available", ErrInvalidHostEnv)
	}

	const gasCost = 15 // Slightly less than address because no hex conversion
	if err := hostEnv.GasTank.ConsumeGas(gasCost); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	if hostEnv.CallerPublicKey == nil || len(hostEnv.CallerPublicKey) == 0 {
		hostEnv.Logger.Warnf("VM_HOST_WARN: BlockchainGetCallerPublicKey: CallerPublicKey not set in host environment. Returning error code.")
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodePublicKeyNotAvailable))}, nil // Return error code to WASM
	}

	actualLen := int32(len(hostEnv.CallerPublicKey)) // Should be 65 for P256 uncompressed
	retBufPtr := args[0].I32()
	retBufLen := args[1].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if actualLen > retBufLen {
		hostEnv.Logger.Warnf("VM_HOST_WARN: BlockchainGetCallerPublicKey: buffer too small. Need %d, have %d\n", actualLen, retBufLen)
	}

	bytesToCopy := min(actualLen, retBufLen)
	if bytesToCopy > 0 {
		if retBufPtr < 0 || int64(retBufPtr)+int64(bytesToCopy) > int64(len(wasmMemoryData)) {
			hostEnv.Logger.Errorf("VM_HOST_ERR: get_caller_public_key: return buffer memory access out of bounds for contract %x", hostEnv.ContractAddress)
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, nil
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], hostEnv.CallerPublicKey[:bytesToCopy])
	}
	hostEnv.Logger.Debugf("VM_HOST: blockchain_get_caller_public_key: returning %d bytes.", actualLen)
	return []wasmer.Value{wasmer.NewI32(actualLen)}, nil // Return actual length
}

// blockchain_generate_did_key (env.blockchain_generate_did_key) - For contracts to generate a did:key string from a pubkey.
// Parameters: pubkey_ptr (i32), pubkey_len (i32), ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_did_string_len (i32)
func BlockchainGenerateDidKey(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("%w: host memory not available", ErrInvalidHostEnv)
	}

	const gasCost = 200 // Higher cost due to cryptographic/encoding ops
	if err := hostEnv.GasTank.ConsumeGas(gasCost); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	pubkeyPtr, pubkeyLen := args[0].I32(), args[1].I32()
	retBufPtr, retBufLen := args[2].I32(), args[3].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	// Validate pubkey memory access
	if pubkeyPtr < 0 || pubkeyLen <= 0 || int64(pubkeyPtr)+int64(pubkeyLen) > int64(len(wasmMemoryData)) {
		hostEnv.Logger.Errorf("VM_HOST_ERR: GenerateDidKey: pubkey memory access out of bounds.")
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, nil
	}
	pubkeyBytes := wasmMemoryData[pubkeyPtr : pubkeyPtr+pubkeyLen]

	// Call the crypto package's DID generation function
	didKeyString, err := crypto.GenerateDIDKeySecp256r1(pubkeyBytes)
	if err != nil {
		hostEnv.Logger.Errorf("VM_HOST_ERR: GenerateDidKey: failed to generate did:key for pubkey %x: %v", pubkeyBytes, err)
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, nil // Return generic failure code to WASM
	}

	didKeyBytes := []byte(didKeyString)
	actualLen := int32(len(didKeyBytes))

	// Validate return buffer and copy DID string
	if actualLen > retBufLen {
		hostEnv.Logger.Warnf("VM_HOST_WARN: GenerateDidKey: buffer too small for DID string. Need %d, have %d\n", actualLen, retBufLen)
	}

	bytesToCopy := min(actualLen, retBufLen)
	if bytesToCopy > 0 {
		if retBufPtr < 0 || int64(retBufPtr)+int64(bytesToCopy) > int64(len(wasmMemoryData)) {
			hostEnv.Logger.Errorf("VM_HOST_ERR: GenerateDidKey: return buffer memory access out of bounds.")
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, nil
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], didKeyBytes[:bytesToCopy])
	}
	hostEnv.Logger.Debugf("VM_HOST: blockchain_generate_did_key: returning DID string of len %d.", actualLen)
	return []wasmer.Value{wasmer.NewI32(actualLen)}, nil // Return actual length of the DID string
}

// BlockchainEmitEvent (env.blockchain_emit_event) - For contracts to emit custom events to the blockchain.
// Parameters: topic_ptr (i32), topic_len (i32), data_ptr (i32), data_len (i32)
// Results: none
func BlockchainEmitEvent(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil {
		return nil, fmt.Errorf("%w: host memory not available for event emission", ErrInvalidHostEnv)
	}

	const gasCostBase = 50 // Base cost for emitting an event
	if err := hostEnv.GasTank.ConsumeGas(gasCostBase); err != nil {
		return []wasmer.Value{}, err
	}

	topicPtr, topicLen := args[0].I32(), args[1].I32()
	dataPtr, dataLen := args[2].I32(), args[3].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	// Validate memory access for topic and data
	if topicPtr < 0 || topicLen < 0 || int64(topicPtr)+int64(topicLen) > int64(len(wasmMemoryData)) ||
		dataPtr < 0 || dataLen < 0 || int64(dataPtr)+int64(dataLen) > int64(len(wasmMemoryData)) {
		hostEnv.Logger.Errorf("VM_HOST_ERR: BlockchainEmitEvent: invalid memory access for contract %x", hostEnv.ContractAddress)
		return nil, fmt.Errorf("%w: memory access out of bounds in host_emit_event", ErrCodeInvalidMemoryAccess)
	}

	gasCostBytes := uint64(topicLen + dataLen) // Gas based on event size
	if err := hostEnv.GasTank.ConsumeGas(gasCostBytes); err != nil {
		return []wasmer.Value{}, err
	}

	topic := string(wasmMemoryData[topicPtr : topicPtr+topicLen])
	eventData := wasmMemoryData[dataPtr : dataPtr+dataLen] // Raw bytes for data, not necessarily string

	// TODO: Actually emit event to a blockchain event bus or state manager for logging/indexing
	hostEnv.Logger.Printf("CONTRACT_EVENT (Addr: %x): Topic: '%s', Data: %x\n", hostEnv.ContractAddress, topic, eventData)
	return []wasmer.Value{}, nil
}

// Helper for min in GetStorage, if not already globally available
func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
