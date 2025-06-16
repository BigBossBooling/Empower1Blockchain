package vm

import (
	"fmt"
	"sync/atomic"
)

// ErrOutOfGas is a sentinel error for when gas is exhausted.
var ErrOutOfGas = fmt.Errorf("execution halted: out of gas")

// GasTank manages gas consumption for an execution context.
type GasTank struct {
	limit      uint64
	consumed   uint64 // Use atomic operations if this needs to be goroutine-safe beyond a single execution
	wasmerCost uint64 // Gas consumed by Wasmer's instruction counting (if available and used)
}

// NewGasTank creates a new gas tank with a given limit.
func NewGasTank(limit uint64) *GasTank {
	return &GasTank{
		limit:    limit,
		consumed: 0,
	}
}

// ConsumeGas attempts to consume a specified amount of gas.
// Returns ErrOutOfGas if consumption exceeds the limit.
func (gt *GasTank) ConsumeGas(amount uint64) error {
	newConsumed := atomic.AddUint64(&gt.consumed, amount) // For potential concurrent host func calls, though usually single threaded exec
	// newConsumed := gt.consumed + amount // If strictly single-threaded execution per contract call

	if newConsumed > gt.limit {
		// Roll back consumption if it exceeds limit to reflect actual state before this op
		atomic.StoreUint64(&gt.consumed, gt.limit) // Set consumed to limit
		// gt.consumed = gt.limit
		return ErrOutOfGas
	}
	// gt.consumed = newConsumed // if not using atomic for main path
	return nil
}

// GasConsumed returns the total amount of gas consumed so far.
func (gt *GasTank) GasConsumed() uint64 {
	return atomic.LoadUint64(&gt.consumed)
	// return gt.consumed
}

// GasLimit returns the initial gas limit.
func (gt *GasTank) GasLimit() uint64 {
	return gt.limit
}

// GasRemaining returns the amount of gas left.
func (gt *GasTank) GasRemaining() uint64 {
	consumed := atomic.LoadUint64(&gt.consumed)
	// consumed := gt.consumed
	if consumed >= gt.limit {
		return 0
	}
	return gt.limit - consumed
}

// SetWasmerCost sets the gas cost as determined by Wasmer's metering middleware.
// This would be called after WASM execution if Wasmer provides this value.
func (gt *GasTank) SetWasmerCost(cost uint64) error {
	// This cost is from the WASM execution itself. Add it to currently consumed gas.
	// This assumes Wasmer cost is separate and needs to be "paid" from the tank.
	return gt.ConsumeGas(cost)
}

// --- Placeholder for Wasmer Metering Middleware Integration ---
// Currently, wasmer-go (v1.0.1) does not directly expose Wasmer's
// metering middleware (`wasmer_middlewares::metering`) in a way that allows
// easy setting of gas costs per instruction block or getting consumed gas count
// directly after execution.
//
// If direct metering were available:
// 1. Before execution: You'd set a gas limit with the middleware.
// 2. During/After execution: The middleware would halt execution if gas limit
//    is exceeded, or you could retrieve the consumed gas from it.
//
// As a workaround or simpler model for now:
// - We manually account for gas in host functions.
// - For WASM instructions, we could:
//   a) Ignore instruction-level gas (simplest, but inaccurate).
//   b) Estimate gas based on bytecode size or complexity (very rough).
//   c) If a future wasmer-go version or another runtime provides it, integrate it.
//   d) For now, `SetWasmerCost` is a placeholder if we could get such a value.
//
// The current `GasTank` primarily tracks gas for host function calls.
// The `VMService.ExecuteContract` will create and manage this tank.
// If a host function returns `ErrOutOfGas`, `ExecuteContract` should propagate it.
