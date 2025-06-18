package vm

import (
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output (optional, assuming central logger)
	"sync/atomic" // For atomic operations on consumed gas
)

// ErrOutOfGas is a sentinel error for when gas is exhausted during smart contract execution.
// This is a critical signal for the VM to halt execution.
var ErrOutOfGas = errors.New("execution halted: out of gas")

// GasTank manages gas consumption for a single smart contract execution context.
// It tracks the allocated gas limit and the amount consumed by WASM instructions
// and host function calls.
type GasTank struct {
	limit            uint64      // The maximum gas allowed for this execution
	consumed         uint64      // Atomic counter for total gas consumed by host functions and base execution
	wasmerInstructionCost uint64 // Gas consumed by Wasmer's *internal* instruction counting (if available)
	logger           *log.Logger // Dedicated logger for the GasTank
}

// NewGasTank creates a new GasTank instance with a given gas limit.
// All contract executions should operate within a specific gas budget.
func NewGasTank(limit uint64) (*GasTank, error) {
	if limit == 0 {
		return nil, fmt.Errorf("%w: gas limit must be positive", ErrVMInit)
	}
	logger := log.New(os.Stdout, "GASTANK: ", log.Ldate|log.Ltime|log.Lshortfile)
	gt := &GasTank{
		limit:    limit,
		consumed: 0,
		logger:   logger,
	}
	gt.logger.Debugf("GasTank initialized with limit: %d", limit)
	return gt, nil
}

// ConsumeGas attempts to consume a specified amount of gas.
// This method should be called by host functions and for base execution costs.
// It returns ErrOutOfGas if consumption exceeds the limit.
func (gt *GasTank) ConsumeGas(amount uint64) error {
	// Atomically add the amount to consumed gas.
	// This ensures thread-safety if host functions were to run concurrently (less common in WASM VMs for single instance).
	newConsumed := atomic.AddUint64(&gt.consumed, amount) 

	if newConsumed > gt.limit {
		// If consumption exceeds limit, "refund" the excess and return ErrOutOfGas.
		// The actual consumed amount should be capped at the limit.
		atomic.StoreUint64(&gt.consumed, gt.limit) 
		gt.logger.Debugf("GASTANK_OOG: Out of gas. Tried to consume %d, total %d, limit %d", amount, newConsumed, gt.limit)
		return ErrOutOfGas
	}
	gt.logger.Debugf("GASTANK: Consumed %d gas. Total: %d/%d", amount, newConsumed, gt.limit)
	return nil
}

// GasConsumed returns the total amount of gas consumed so far, including
// host function costs and any accounted WASM instruction costs.
func (gt *GasTank) GasConsumed() uint64 {
	return atomic.LoadUint64(&gt.consumed) // Atomically load current consumed value
}

// GasLimit returns the initial gas limit set for this execution.
func (gt *GasTank) GasLimit() uint64 {
	return gt.limit
}

// GasRemaining returns the amount of gas left in the tank.
// It will be 0 if the limit has been reached or exceeded.
func (gt *GasTank) GasRemaining() uint64 {
	consumed := atomic.LoadUint64(&gt.consumed)
	if consumed >= gt.limit {
		return 0
	}
	return gt.limit - consumed
}

// SetWasmerInstructionCost is a conceptual function to integrate Wasmer's
// internal instruction metering cost. This would be called by the VMService
// after the WASM execution completes and Wasmer reports its own consumed gas.
func (gt *GasTank) SetWasmerInstructionCost(cost uint64) error {
	// This function *adds* Wasmer's reported instruction cost to the tank.
	// It assumes Wasmer's metering middleware accounts for the WASM instructions themselves,
	// independent of host function costs.
	gt.wasmerInstructionCost = cost // Store the cost reported by Wasmer
	
	// Then, "consume" this cost from the tank. If it pushes us over limit, then OutOfGas.
	// This integrates Wasmer's cost into our overall gas budget.
	return gt.ConsumeGas(cost)
}

// GetWasmerInstructionCost returns the cost as reported by Wasmer's internal metering.
// This is for auditing/logging purposes.
func (gt *GasTank) GetWasmerInstructionCost() uint64 {
	return gt.wasmerInstructionCost
}

// --- Strategic Rationale for Gas Metering ---
// Adhering to "Sense the Landscape, Secure the Solution" for resource management.
// This comprehensive GasTank implementation ensures:
// 1. DoS Prevention: Limits computational resources consumed by contracts.
// 2. Predictability: Allows users/developers to estimate execution costs.
// 3. Fairness: Ensures complex operations pay their fair share of network resources.
// 4. Integrability: Provides clear hooks for both host function and (conceptual) WASM instruction metering.
// 5. Auditability: Logs gas consumption for transparency.