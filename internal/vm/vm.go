package vm

import (
	"fmt"
	"github.com/wasmerio/wasmer-go/wasmer"
	// "log" // Removed unused import
	// "empower1.com/core/internal/core"
)

// VMService is responsible for executing WASM smart contracts.
type VMService struct {
	// engine could be stored here if we want to reuse it
}

// NewVMService creates a new VMService.
func NewVMService() *VMService {
	return &VMService{}
}

// ExecuteContract loads and runs a WASM contract.
func (vms *VMService) ExecuteContract(
	contractAddress []byte, // Address of the contract being executed
	callerPublicKey []byte, // Raw public key of the transaction signer, for hostEnv
	wasmCode []byte,
	functionName string,
	gasLimit uint64,
	args ...interface{}) (result interface{}, gasConsumed uint64, err error) {

	gasTank := NewGasTank(gasLimit)
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	module, err := wasmer.NewModule(store, wasmCode)
	if err != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("failed to compile WASM module: %w", err)
	}

	hostEnv := &HostFunctionEnvironment{
		ContractAddress: contractAddress,
		GasTank:         gasTank,
		CallerPublicKey: callerPublicKey, // Set caller's public key in the environment
		// Memory will be set after instance creation
	}

	importObject := wasmer.NewImportObject()
	envImports := map[string]wasmer.IntoExtern{
		// Storage
		"blockchain_set_storage": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType( // Args: keyPtr, keyLen, valPtr, valLen (all I32)
				wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32),
				wasmer.NewValueTypes(wasmer.I32), // Results: errCode (I32)
			), hostEnv, BlockchainSetStorage),
		"blockchain_get_storage": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType( // Args: keyPtr, keyLen, retBufPtr, retBufLen (all I32)
				wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32),
				wasmer.NewValueTypes(wasmer.I32), // Results: actualValLen (I32)
			), hostEnv, BlockchainGetStorage),

		// Logging & Events
		"host_log_message": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType( // Args: msgPtr, msgLen (I32, I32)
				wasmer.NewValueTypes(wasmer.I32, wasmer.I32),
				wasmer.NewValueTypes(), // Results: none
			), hostEnv, BlockchainLogMessage),
		"blockchain_emit_event": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType( // Args: topicPtr, topicLen, dataPtr, dataLen (all I32)
				wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32),
				wasmer.NewValueTypes(), // Results: none
			), hostEnv, BlockchainEmitEvent),

		// Caller/Context Info
		"blockchain_get_caller_public_key": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType( // Args: retBufPtr, retBufLen (I32, I32)
				wasmer.NewValueTypes(wasmer.I32, wasmer.I32),
				wasmer.NewValueTypes(wasmer.I32), // Results: actualKeyLen (I32)
			), hostEnv, BlockchainGetCallerPublicKey),
		"blockchain_generate_did_key": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType( // Args: pubKeyPtr, pubKeyLen, retBufPtr, retBufLen (all I32)
				wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32),
				wasmer.NewValueTypes(wasmer.I32), // Results: actualDIDLen (I32)
			), hostEnv, BlockchainGenerateDidKey),

		// Other stubs (ensure signatures match host_functions.go)
		"blockchain_get_caller_address": wasmer.NewFunctionWithEnvironment(store, // Still returns hex string via buffer
			wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainGetCallerAddress),
		"blockchain_get_balance": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32), wasmer.NewValueTypes(wasmer.I64)), hostEnv, BlockchainGetBalance),
		"blockchain_send_funds": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I64), wasmer.NewValueTypes(wasmer.I32)), hostEnv, BlockchainSendFunds),
		"blockchain_get_block_timestamp": wasmer.NewFunctionWithEnvironment(store,
			wasmer.NewFunctionType(wasmer.NewValueTypes(), wasmer.NewValueTypes(wasmer.I64)), hostEnv, BlockchainGetBlockTimestamp),
	}
	importObject.Register("env", envImports)

	const baseWasmExecutionCost = 100
	if errGas := gasTank.ConsumeGas(baseWasmExecutionCost); errGas != nil {
		return nil, gasTank.GasConsumed(), ErrOutOfGas
	}

	instance, errInstance := wasmer.NewInstance(module, importObject)
	if errInstance != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("failed to instantiate WASM module: %w", errInstance)
	}
	// No defer instance.Close() needed as per previous correction

	memory, errMemory := instance.Exports.GetMemory("memory")
	if errMemory != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("failed to get exported WASM memory: %w", errMemory)
	}
	hostEnv.Memory = memory // Link memory to host environment for functions to use

	wasmFunc, errFunc := instance.Exports.GetFunction(functionName)
	if errFunc != nil {
		return nil, gasTank.GasConsumed(), fmt.Errorf("failed to get exported WASM function '%s': %w", functionName, errFunc)
	}

	rawResult, errExec := wasmFunc(args...)
	if errExec != nil {
		if errExec == ErrOutOfGas { // Check if error is our specific ErrOutOfGas
			return nil, gasTank.GasConsumed(), ErrOutOfGas
		}
		// Check if it's a Wasmer Trap due to out of gas from internal metering (if it were active)
		// This is a best guess; specific error types from Wasmer would be better.
		if _, ok := errExec.(*wasmer.TrapError); ok {
			// A trap could be due to various reasons, including out-of-gas if Wasmer itself was metering.
			// For now, assume host function gas is primary. If Wasmer traps, it could be other runtime error.
			// If we had Wasmer's own gas value, we'd use it here.
			// This might need more specific error checking for Wasmer traps.
			if gasTank.GasRemaining() == 0 { // If our tank is also empty, likely related
				return nil, gasTank.GasConsumed(), ErrOutOfGas
			}
		}
		return nil, gasTank.GasConsumed(), fmt.Errorf("error calling WASM function '%s': %w", functionName, errExec)
	}

	if rawResult == nil {
		return nil, gasTank.GasConsumed(), nil
	}
	return rawResult, gasTank.GasConsumed(), nil
}
