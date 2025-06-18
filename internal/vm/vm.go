package vm

import (
	"bytes"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output
	"time"   // For potential future timeouts or execution tracking

	"github.com/wasmerio/wasmer-go/wasmer"

	"empower1.com/core/core"  // Assuming this path to core package for Block, Transaction types etc.
	"empower1.com/core/state" // Assuming this path to state package for state.State interaction
	"empower1.com/core/crypto" // Assuming this path for crypto functions like DID generation
)

// --- Custom Errors for VM Service ---
var (
	ErrVMInit                  = errors.New("vm service initialization error")
	ErrWASMCompile             = errors.New("failed to compile WASM module")
	ErrWASMInstantiate         = errors.New("failed to instantiate WASM module")
	ErrWASMExportMissing       = errors.New("missing WASM export (function or memory)")
	ErrWASMExecution           = errors.New("wasm function execution failed")
	ErrOutOfGas                = errors.New("out of gas during WASM execution") // Defined in gas.go
	ErrHostFunctionExecution   = errors.New("host function execution failed")
	ErrInvalidHostEnv          = errors.New("invalid host environment for WASM call")
	ErrInvalidCallerKey        = errors.New("invalid caller public key provided")
)

// VMService is responsible for executing WASM smart contracts.
// It manages the Wasmer engine and store, providing a clean interface for contract execution.
type VMService struct {
	// For reusability, a Wasmer engine and store *could* be stored here,
	// but often, they are created per execution context for isolation.
	// For V1, creating a new engine/store per execution is simpler and safer.
	logger *log.Logger // Dedicated logger for the VMService
	state  *state.State // Reference to the global blockchain state for host functions
}

// NewVMService creates a new VMService instance.
// It requires a reference to the global blockchain state for host function interactions.
func NewVMService(state *state.State) (*VMService, error) {
	if state == nil {
		return nil, fmt.Errorf("%w: state manager cannot be nil", ErrVMInit)
	}
	logger := log.New(os.Stdout, "VM_SERVICE: ", log.Ldate|log.Ltime|log.Lshortfile)
	vms := &VMService{
		logger: logger,
		state:  state,
	}
	vms.logger.Println("VMService initialized.")
	return vms, nil
}

// --- Host Function Environment (Passed to WASM modules) ---
// This struct makes blockchain state and context available to WASM smart contracts.
type HostFunctionEnvironment struct {
	ContractAddress []byte          // Address of the contract currently being executed
	CallerPublicKey []byte          // Public key bytes of the caller/signer of the transaction
	GasTank         *GasTank        // Reference to the gas metering mechanism
	Memory          *wasmer.Memory  // WASM instance memory (set after instance creation)
	Instance        *wasmer.Instance // Reference to the WASM instance itself (for internal calls, V2+)
	Logger          *log.Logger     // Logger for host functions
	State           *state.State    // Reference to the global blockchain state
}

// Ensure interface compliance for Wasmer environment
var _ wasmer.WasmerEnv = &HostFunctionEnvironment{}

// Crate new HostFunctionEnvironment for each contract execution
func NewHostFunctionEnvironment(contractAddr, callerPubKey []byte, gasTank *GasTank, state *state.State) (*HostFunctionEnvironment, error) {
	if len(contractAddr) == 0 || len(callerPubKey) == 0 || gasTank == nil || state == nil {
		return nil, ErrInvalidHostEnv
	}
	return &HostFunctionEnvironment{
		ContractAddress: contractAddr,
		CallerPublicKey: callerPubKey,
		GasTank:         gasTank,
		Logger:          log.New(os.Stdout, "HOST_FN: ", log.Ldate|log.Ltime|log.Lshortfile),
		State:           state,
	}, nil
}

// OnInstantiated is part of wasmer.WasmerEnv interface.
// It's called by Wasmer after the module is instantiated, allowing us to link memory.
func (env *HostFunctionEnvironment) OnInstantiated(instance *wasmer.Instance) error {
	memory, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return fmt.Errorf("failed to get exported WASM memory during instantiation: %w", err)
	}
	env.Memory = memory
	env.Instance = instance // Store instance for potential inter-contract calls
	return nil
}

// --- WASM Host Functions (Defined in host_functions.go, referenced here) ---
// These are the "system calls" that a smart contract can make to interact with the blockchain.
// Their signatures must precisely match the WASM function types.

// Placeholder for actual host functions, assuming they are in 'host_functions.go'
// These are included here conceptually to show how VMService registers them.
// A real implementation would ensure these functions are defined and imported.

// BlockchainSetStorage (keyPtr, keyLen, valPtr, valLen) -> (errCode)
func BlockchainSetStorage(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI32(0)}, nil // Dummy success
}
// BlockchainGetStorage (keyPtr, keyLen, retBufPtr, retBufLen) -> (actualValLen)
func BlockchainGetStorage(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI32(0)}, nil // Dummy length
}
// BlockchainLogMessage (msgPtr, msgLen) -> ()
func BlockchainLogMessage(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{}, nil
}
// BlockchainEmitEvent (topicPtr, topicLen, dataPtr, dataLen) -> ()
func BlockchainEmitEvent(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{}, nil
}
// BlockchainGetCallerPublicKey (retBufPtr, retBufLen) -> (actualKeyLen)
func BlockchainGetCallerPublicKey(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI32(0)}, nil // Dummy length
}
// BlockchainGenerateDidKey (pubKeyPtr, pubKeyLen, retBufPtr, retBufLen) -> (actualDIDLen)
func BlockchainGenerateDidKey(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI32(0)}, nil // Dummy length
}
// BlockchainGetCallerAddress (retBufPtr, retBufLen) -> (actualLen)
func BlockchainGetCallerAddress(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI32(0)}, nil
}
// BlockchainGetBalance (addressPtr, addressLen) -> (balance)
func BlockchainGetBalance(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI64(0)}, nil
}
// BlockchainSendFunds (toAddrPtr, toAddrLen, amount) -> (errCode)
func BlockchainSendFunds(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI32(0)}, nil
}
// BlockchainGetBlockTimestamp () -> (timestamp)
func BlockchainGetBlockTimestamp(envPtr wasmer.IntoWasmValue, args []wasmer.IntoWasmValue) ([]wasmer.IntoWasmValue, error) {
    // ... actual implementation from host_functions.go ...
    return []wasmer.IntoWasmValue{wasmer.NewI64(time.Now().UnixNano())}, nil
}


// ExecuteContract loads and runs a WASM contract.
// It manages the Wasmer lifecycle (engine, store, module, instance) per execution for isolation.
// This ensures secure and predictable contract execution.
func (vms *VMService) ExecuteContract(
	contractAddress []byte, // Address of the contract being executed
	callerPublicKey []byte, // Raw public key of the transaction signer/caller
	wasmCode []byte,        // WASM bytecode of the contract
	functionName string,    // Name of the exported WASM function to call
	gasLimit uint64,        // Maximum gas allowed for this execution
	args ...wasmer.IntoWasmValue, // Arguments to pass to the WASM function
) (result interface{}, gasConsumed uint64, err error) {

	// Initialize GasTank for this execution, adhering to "Know Your Core, Keep it Clear" for resource management.
	gasTank := NewGasTank(gasLimit)
	
	// Create new Wasmer engine and store for each execution for isolation and security.
	// This ensures clean state for each contract call, crucial for integrity.
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	defer store.Close() // Ensure store resources are released

	// Compile the WASM module. This can be cached in a production system for efficiency.
	module, err := wasmer.NewModule(store, wasmCode)
	if err != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("%w: %v", ErrWASMCompile, err)
	}
	defer module.Close() // Ensure module resources are released

	// Create the HostFunctionEnvironment for this execution.
	// This links the contract's execution to the blockchain's state and context.
	hostEnv, err := NewHostFunctionEnvironment(contractAddress, callerPublicKey, gasTank, vms.state)
	if err != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("%w: failed to create host environment: %v", ErrVMInit, err)
	}

	// Register host functions (imports) for the WASM module.
	importObject := wasmer.NewImportObject()
	envImports := map[string]wasmer.IntoExtern{
		"blockchain_set_storage":         wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainSetStorage),
		"blockchain_get_storage":         wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainGetStorage),
		"host_log_message":               wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32), wasmer.NewValueTypes()), hostEnv, BlockchainLogMessage),
		"blockchain_emit_event":          wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes()), hostEnv, BlockchainEmitEvent),
		"blockchain_get_caller_public_key": wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainGetCallerPublicKey),
		"blockchain_generate_did_key":    wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainGenerateDidKey),
		"blockchain_get_caller_address":  wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainGetCallerAddress),
		"blockchain_get_balance":         wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I64)), hostEnv, BlockchainGetBalance),
		"blockchain_send_funds":          wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I64), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainSendFunds),
		"blockchain_get_block_timestamp": wasmer.NewFunctionWithEnvironment(store, wasmer.NewFunctionType(wasmer.NewValueTypes(), wasmer.NewValueTypes(wasmer.I64)), hostEnv, BlockchainGetBlockTimestamp),
	}
	importObject.Register("env", envImports)

	// Base cost for WASM execution, adhering to "Sense the Landscape, Secure the Solution" for resource management.
	const baseWasmExecutionCost = 100 // Example base cost, tune for actual complexity
	if errGas := gasTank.ConsumeGas(baseWasmExecutionCost); errGas != nil {
		vms.logger.Warnf("VM: Base execution cost failed for contract %x: %v", contractAddress, errGas)
		return nil, gasTank.GasConsumed(), ErrOutOfGas // Return specific OOG error
	}

	instance, errInstance := wasmer.NewInstance(module, importObject)
	if errInstance != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("%w: failed to instantiate WASM module for contract %x: %v", ErrWASMInstantiate, contractAddress, errInstance)
	}
	defer instance.Close() // Ensure instance resources are released after execution

	// Link memory to host environment (done by OnInstantiated, but ensure it's not nil)
	if hostEnv.Memory == nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("%w: WASM memory not found after instantiation for contract %x", ErrWASMExportMissing, contractAddress)
	}

	// Get the exported WASM function to call.
	wasmFunc, errFunc := instance.Exports.GetFunction(functionName)
	if errFunc != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("%w: failed to get exported WASM function '%s' for contract %x: %v", ErrWASMExportMissing, functionName, contractAddress, errFunc)
	}

	// Execute the WASM function.
	rawResult, errExec := wasmFunc(args...) // Pass arguments directly
	if errExec != nil {
		// Log detailed error from WASM execution.
		vms.logger.Errorf("VM: Error calling WASM function '%s' for contract %x: %v", functionName, contractAddress, errExec)
		
		// Differentiate OutOfGas from other WASM traps/runtime errors.
		// If our GasTank is empty, it's an OutOfGas. Wasmer TrapError could also be OOG if metering within Wasmer.
		if gasTank.GasRemaining() == 0 { 
			return nil, gasTank.GasConsumed(), ErrOutOfGas
		}
		if _, ok := errExec.(*wasmer.TrapError); ok {
			// A trap could be due to various reasons (e.g., division by zero, out-of-bounds memory access)
			// not just out of gas. This is a general WASM runtime error.
			return nil, gasTank.GasConsumed(), fmt.Errorf("%w: WASM runtime trap during '%s' for contract %x: %v", ErrWASMExecution, functionName, contractAddress, errExec)
		}
		return nil, gasTank.GasConsumed(), fmt.Errorf("%w: unexpected error during WASM function call '%s' for contract %x: %v", ErrWASMExecution, functionName, contractAddress, errExec)
	}

	// Process raw result. WASM functions can return multiple values or no values.
	var finalResult interface{}
	if len(rawResult) > 0 {
		finalResult = rawResult[0] // Assuming single return value for simplicity
	} else {
		finalResult = nil // No return value
	}
	
	vms.logger.Printf("VM: Executed contract %x, function '%s'. Gas Used: %d", contractAddress, functionName, gasTank.GasConsumed())
	return finalResult, gasTank.GasConsumed(), nil
}