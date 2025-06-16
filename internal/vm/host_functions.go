package vm

import (
	"empower1.com/core/internal/crypto" // For crypto.GenerateDIDKey
	"empower1.com/core/internal/state"
	"encoding/hex" // For BlockchainGetCallerAddress
	"fmt"
	"github.com/wasmerio/wasmer-go/wasmer"
	"log"
	"time" // For BlockchainGetBlockTimestamp
)

// HostFunctionEnvironment provides access to elements needed by host functions,
// like the WASM memory or the contract's address.
type HostFunctionEnvironment struct {
	Memory          *wasmer.Memory
	ContractAddress []byte // Address of the currently executing contract
	GasTank         *GasTank // Pointer to the gas tank for this execution
	// Add other context here, e.g., CallerAddress, BlockHeight, etc.
	// CurrentBlock *core.Block // Example: if host functions need block context
	CallerPublicKey []byte // Raw public key of the original transaction signer
}

const (
	ErrCodeSuccess uint32 = 0
	ErrCodeFailure uint32 = 1 // Generic failure
	// ErrCodeKeyNotFound uint32 = 2 // Not used by get_storage currently, it returns 0 length for not found
	ErrCodeInvalidMemoryAccess   uint32 = 3
	ErrCodeBufferTooSmall        uint32 = 4
	ErrCodeOutOfGas              uint32 = 5
	ErrCodePublicKeyNotAvailable uint32 = 6 // If caller public key cannot be determined
	ErrCodeBadArgument           uint32 = 7 // If WASM provides bad arguments to host function
)

// --- Host Function Implementations ---

// blockchain_log_message (env.host_log_message)
// Parameters: message_ptr (i32), message_len (i32)
// Results: none (AssemblyScript expects void, so empty results array)
func BlockchainLogMessage(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil {
		log.Println("VM_HOST_ERR: BlockchainLogMessage: memory not available in environment!")
		return nil, fmt.Errorf("host_log_message: host memory not configured") // Host error, not WASM error code
	}

	const gasCostLogMessageBase = 10
	if err := hostEnv.GasTank.ConsumeGas(gasCostLogMessageBase); err != nil {
		return []wasmer.Value{}, err // Propagate ErrOutOfGas; WASM expects no return values for success
	}

	messagePtr := args[0].I32()
	messageLen := args[1].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if messagePtr < 0 || messageLen < 0 || int64(messagePtr+messageLen) > int64(len(wasmMemoryData)) {
		log.Printf("VM_HOST_ERR: BlockchainLogMessage: invalid memory access ptr=%d, len=%d, mem_size=%d for contract %x\n", messagePtr, messageLen, len(wasmMemoryData), hostEnv.ContractAddress)
		// This case should ideally return an error that the WASM contract can understand,
		// but logTest in AS doesn't expect a return value. For now, host logs error and continues.
		// If the host function signature in AS *did* return an error code, we'd do:
		// return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: memory access out of bounds in host_log_message")
		return []wasmer.Value{}, fmt.Errorf("wasm: memory access out of bounds in host_log_message") // Still return error to halt if fatal
	}

	gasCostPerByte := uint64(1) // Example: 1 gas per byte of log message
	if err := hostEnv.GasTank.ConsumeGas(uint64(messageLen) * gasCostPerByte); err != nil {
		return []wasmer.Value{}, err
	}

	message := string(wasmMemoryData[messagePtr : messagePtr+messageLen])
	log.Printf("CONTRACT_LOG (Addr: %x): %s\n", hostEnv.ContractAddress, message)

	return []wasmer.Value{}, nil
}


// blockchain_set_storage (env.blockchain_set_storage in AS)
// Parameters: key_ptr (i32), key_len (i32), value_ptr (i32), value_len (i32)
// Results: error_code (i32) -> ErrCodeSuccess (0) for success
func BlockchainSetStorage(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("host_set_storage: memory not available") }
	if hostEnv.ContractAddress == nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("host_set_storage: contract address not available") }

	const gasCostSetStorageBase = 100
	if err := hostEnv.GasTank.ConsumeGas(gasCostSetStorageBase); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	keyPtr := args[0].I32()
	keyLen := args[1].I32()
	valPtr := args[2].I32()
	valLen := args[3].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if keyPtr < 0 || keyLen < 0 || int64(keyPtr+keyLen) > int64(len(wasmMemoryData)) {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: key memory access out of bounds in host_set_storage")
	}
	key := make([]byte, keyLen)
	copy(key, wasmMemoryData[keyPtr:keyPtr+keyLen])

	if valPtr < 0 || valLen < 0 || int64(valPtr+valLen) > int64(len(wasmMemoryData)) {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: value memory access out of bounds in host_set_storage")
	}
	value := make([]byte, valLen)
	copy(value, wasmMemoryData[valPtr:valPtr+valLen])

	gasCostStorageBytes := uint64(keyLen + valLen)
	if err := hostEnv.GasTank.ConsumeGas(gasCostStorageBytes); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	err := state.SetContractStorage(hostEnv.ContractAddress, key, value)
	if err != nil {
		log.Printf("VM_HOST_ERR: host_set_storage failed for contract %x, key %x: %v\n", hostEnv.ContractAddress, key, err)
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("host_set_storage: state interaction failed: %w", err)
	}
	return []wasmer.Value{wasmer.NewI32(int32(ErrCodeSuccess))}, nil
}


// blockchain_get_storage (env.blockchain_get_storage in AS)
// Parameters: key_ptr (i32), key_len (i32), ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_value_len (i32).
//          - If > ret_buf_len, buffer was too small, data up to ret_buf_len copied.
//          - If == 0 and value existed but was empty, it's 0.
//          - If key not found, returns 0.
//          - Negative values could be specific error codes (e.g. -ErrCodeInvalidMemoryAccess).
// For simplicity and matching common patterns, let's stick to:
// Returns actual length of the value. If key not found, returns 0.
// If buffer too small, it copies min(actual_len, ret_buf_len) and returns actual_len.
// The contract checks if actual_len > ret_buf_len.
func BlockchainGetStorage(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("host_get_storage: memory not available") }
	if hostEnv.ContractAddress == nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("host_get_storage: contract address not available") }

	const gasCostGetStorageBase = 50
	if err := hostEnv.GasTank.ConsumeGas(gasCostGetStorageBase); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	keyPtr := args[0].I32()
	keyLen := args[1].I32()
	retBufPtr := args[2].I32()
	retBufLen := args[3].I32()

	wasmMemoryData := hostEnv.Memory.Data()

	if keyPtr < 0 || keyLen < 0 || int64(keyPtr+keyLen) > int64(len(wasmMemoryData)) {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: key memory access out of bounds in host_get_storage")
	}
	key := make([]byte, keyLen)
	copy(key, wasmMemoryData[keyPtr:keyPtr+keyLen])

	gasCostKeyBytes := uint64(keyLen)
	if err := hostEnv.GasTank.ConsumeGas(gasCostKeyBytes); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	value, err := state.GetContractStorage(hostEnv.ContractAddress, key)
	if err != nil {
		log.Printf("VM_HOST_ERR: host_get_storage state error for contract %x, key %x: %v\n", hostEnv.ContractAddress, key, err)
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("host_get_storage: state interaction failed: %w", err)
	}

	if value == nil { // Key not found
		return []wasmer.Value{wasmer.NewI32(0)}, nil // Return 0 for actual_value_len
	}

	actualValueLen := int32(len(value))

	gasCostValueBytes := uint64(actualValueLen) // Gas for the size of data retrieved
	if err := hostEnv.GasTank.ConsumeGas(gasCostValueBytes); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	// Check buffer validity for writing, even if only copying a part
	bytesToCopy := actualValueLen
	if actualValueLen > retBufLen {
		bytesToCopy = retBufLen // Only copy what fits
		log.Printf("VM_HOST_WARN: host_get_storage: buffer too small for key %x contract %x. Need %d, have %d. Value will be truncated.\n", key, hostEnv.ContractAddress, actualValueLen, retBufLen)
	}

	if bytesToCopy > 0 { // Only attempt copy if there's something to copy
		if retBufPtr < 0 || int64(retBufPtr+bytesToCopy) > int64(len(wasmMemoryData)) {
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: return buffer memory access out of bounds in host_get_storage")
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], value[:bytesToCopy])
	}

	return []wasmer.Value{wasmer.NewI32(actualValueLen)}, nil // Return the *actual* total length of the value.
}


// Placeholder host functions (stubs) - to be fully implemented or connected to blockchain state
func BlockchainGetBalance(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if err := hostEnv.GasTank.ConsumeGas(100); err != nil { return []wasmer.Value{wasmer.NewI64(0)}, err } // Return type is u64
	log.Println("VM_HOST_STUB: blockchain_get_balance called")
	// TODO: Read address from WASM memory, lookup balance, return u64
	return []wasmer.Value{wasmer.NewI64(uint64(0))}, nil
}

func BlockchainSendFunds(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if err := hostEnv.GasTank.ConsumeGas(200); err != nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err } // Return type is i32 (error code)
	log.Println("VM_HOST_STUB: blockchain_send_funds called")
	// TODO: Read params, perform transfer logic
	return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, nil
}

func BlockchainGetBlockTimestamp(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if err := hostEnv.GasTank.ConsumeGas(5); err != nil { return []wasmer.Value{wasmer.NewI64(0)}, err }
	log.Println("VM_HOST_STUB: blockchain_get_block_timestamp called")
	// TODO: Get from current block context in HostFunctionEnvironment
	return []wasmer.Value{wasmer.NewI64(uint64(time.Now().UnixNano()))}, nil // Dummy timestamp
}

// blockchain_get_caller_address (env.blockchain_get_caller_address in AS)
// Parameters: ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_len (i32) of the caller address (hex string of public key)
func BlockchainGetCallerAddress(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("get_caller_address: memory not available") }

	const gasCostGetCaller = 20
	if err := hostEnv.GasTank.ConsumeGas(gasCostGetCaller); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	if hostEnv.CallerPublicKey == nil || len(hostEnv.CallerPublicKey) == 0 {
		log.Println("VM_HOST_ERR: BlockchainGetCallerAddress: CallerPublicKey not set in host environment.")
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodePublicKeyNotAvailable))}, nil
	}

	// The "address" in our system is the hex string of the uncompressed public key.
	callerAddressHex := hex.EncodeToString(hostEnv.CallerPublicKey)
	callerAddressBytes := []byte(callerAddressHex)
	actualLen := int32(len(callerAddressBytes))

	retBufPtr := args[0].I32()
	retBufLen := args[1].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if actualLen > retBufLen {
		log.Printf("VM_HOST_WARN: BlockchainGetCallerAddress: buffer too small. Need %d, have %d\n", actualLen, retBufLen)
		// Return actual length needed, contract can recall with larger buffer.
		// Or, follow convention of returning ErrCodeBufferTooSmall and contract queries for size.
		// For now, return actualLen, and copy what fits.
	}

	bytesToCopy := min(actualLen, retBufLen)
	if bytesToCopy > 0 {
		if retBufPtr < 0 || int64(retBufPtr+bytesToCopy) > int64(len(wasmMemoryData)) {
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: get_caller_address return buffer memory access out of bounds")
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], callerAddressBytes[:bytesToCopy])
	}

	log.Printf("VM_HOST: blockchain_get_caller_address: returning address %s (len %d)\n", callerAddressHex, actualLen)
	return []wasmer.Value{wasmer.NewI32(actualLen)}, nil
}


// blockchain_get_caller_public_key (env.blockchain_get_caller_public_key in AS)
// Parameters: ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_len (i32) of the raw public key bytes
func BlockchainGetCallerPublicKey(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("get_caller_public_key: memory not available") }

	const gasCost = 15
	if err := hostEnv.GasTank.ConsumeGas(gasCost); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	if hostEnv.CallerPublicKey == nil || len(hostEnv.CallerPublicKey) == 0 {
		log.Println("VM_HOST_ERR: BlockchainGetCallerPublicKey: CallerPublicKey not set in host environment.")
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodePublicKeyNotAvailable))}, nil
	}

	actualLen := int32(len(hostEnv.CallerPublicKey))
	retBufPtr := args[0].I32()
	retBufLen := args[1].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if actualLen > retBufLen {
		log.Printf("VM_HOST_WARN: BlockchainGetCallerPublicKey: buffer too small. Need %d, have %d\n", actualLen, retBufLen)
	}

	bytesToCopy := min(actualLen, retBufLen)
	if bytesToCopy > 0 {
		if retBufPtr < 0 || int64(retBufPtr+bytesToCopy) > int64(len(wasmMemoryData)) {
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: get_caller_public_key return buffer memory access out of bounds")
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], hostEnv.CallerPublicKey[:bytesToCopy])
	}
	// log.Printf("VM_HOST: blockchain_get_caller_public_key: returning %d bytes.\n", actualLen)
	return []wasmer.Value{wasmer.NewI32(actualLen)}, nil
}

// blockchain_generate_did_key (env.blockchain_generate_did_key in AS)
// Parameters: pubkey_ptr (i32), pubkey_len (i32), ret_buf_ptr (i32), ret_buf_len (i32)
// Results: actual_did_string_len (i32)
func BlockchainGenerateDidKey(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil { return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, fmt.Errorf("generate_did_key: memory not available") }

	const gasCost = 200 // Slightly higher due to crypto ops
	if err := hostEnv.GasTank.ConsumeGas(gasCost); err != nil {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeOutOfGas))}, err
	}

	pubkeyPtr := args[0].I32()
	pubkeyLen := args[1].I32()
	retBufPtr := args[2].I32()
	retBufLen := args[3].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if pubkeyPtr < 0 || pubkeyLen <= 0 || int64(pubkeyPtr+pubkeyLen) > int64(len(wasmMemoryData)) {
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: pubkey memory access out of bounds in generate_did_key")
	}
	if pubkeyLen != 65 { // Expecting uncompressed P256 key
		log.Printf("VM_HOST_ERR: BlockchainGenerateDidKey: Unexpected pubkey length %d, expected 65.", pubkeyLen)
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeBadArgument))}, nil
	}
	pubkeyBytes := make([]byte, pubkeyLen)
	copy(pubkeyBytes, wasmMemoryData[pubkeyPtr:pubkeyPtr+pubkeyLen])

	didKeyString, err := crypto.GenerateDIDKey(pubkeyBytes)
	if err != nil {
		log.Printf("VM_HOST_ERR: BlockchainGenerateDidKey: failed to generate did:key: %v", err)
		return []wasmer.Value{wasmer.NewI32(int32(ErrCodeFailure))}, nil
	}

	didKeyBytes := []byte(didKeyString)
	actualLen := int32(len(didKeyBytes))

	if actualLen > retBufLen {
		log.Printf("VM_HOST_WARN: BlockchainGenerateDidKey: buffer too small for DID string. Need %d, have %d\n", actualLen, retBufLen)
	}

	bytesToCopy := min(actualLen, retBufLen)
	if bytesToCopy > 0 {
		if retBufPtr < 0 || int64(retBufPtr+bytesToCopy) > int64(len(wasmMemoryData)) {
			return []wasmer.Value{wasmer.NewI32(int32(ErrCodeInvalidMemoryAccess))}, fmt.Errorf("wasm: generate_did_key return buffer memory access out of bounds")
		}
		copy(wasmMemoryData[retBufPtr:retBufPtr+bytesToCopy], didKeyBytes[:bytesToCopy])
	}
	// log.Printf("VM_HOST: blockchain_generate_did_key: returning did string of len %d.\n", actualLen)
	return []wasmer.Value{wasmer.NewI32(actualLen)}, nil
}


func BlockchainEmitEvent(env interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
	hostEnv := env.(*HostFunctionEnvironment)
	if hostEnv.Memory == nil { return nil, fmt.Errorf("host_emit_event: memory not available") }

	const gasCostBase = 50
	if err := hostEnv.GasTank.ConsumeGas(gasCostBase); err != nil { return []wasmer.Value{}, err }

	topicPtr := args[0].I32(); topicLen := args[1].I32()
	dataPtr := args[2].I32(); dataLen := args[3].I32()
	wasmMemoryData := hostEnv.Memory.Data()

	if topicPtr < 0 || topicLen < 0 || int64(topicPtr+topicLen) > int64(len(wasmMemoryData)) ||
		dataPtr < 0 || dataLen < 0 || int64(dataPtr+dataLen) > int64(len(wasmMemoryData)) {
		log.Printf("VM_HOST_ERR: BlockchainEmitEvent: invalid memory access for contract %x\n", hostEnv.ContractAddress)
		// This error should ideally be returned to WASM if the signature allowed, but it's void.
		return []wasmer.Value{}, fmt.Errorf("wasm: memory access out of bounds in host_emit_event")
	}

	gasCostBytes := uint64(topicLen + dataLen)
	if err := hostEnv.GasTank.ConsumeGas(gasCostBytes); err != nil { return []wasmer.Value{}, err }

	topic := string(wasmMemoryData[topicPtr : topicPtr+topicLen])
	eventData := string(wasmMemoryData[dataPtr : dataPtr+dataLen])
	log.Printf("CONTRACT_EVENT (Addr: %x): Topic: '%s', Data: '%s'\n", hostEnv.ContractAddress, topic, eventData)
	return []wasmer.Value{}, nil
}

// Helper for min in GetStorage, if not already globally available
func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
