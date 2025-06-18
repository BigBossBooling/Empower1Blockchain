package consensus

import (
	"bytes" // Used for bytes.Equal for []byte comparison
	"context"
	"errors" // For custom errors
	"fmt"    // For error messages
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"empower1.com/core/core"
)

// Validator-specific errors
var (
	ErrInvalidValidatorAddress = errors.New("validator address cannot be empty")
	ErrInvalidValidatorStake   = errors.New("validator stake must be positive")

	ErrEngineAlreadyRunning = errors.New("consensus engine is already running")
	ErrEngineNotRunning     = errors.New("consensus engine is not running")
	ErrInvalidEngineConfig  = errors.New("invalid consensus engine configuration")
	ErrFailedToGetLastBlock = errors.New("failed to get last block from blockchain")
	ErrFailedToGetProposer  = errors.New("failed to get proposer for height")
	ErrProposeBlockFailed   = errors.New("failed to propose block")
	ErrIncomingBlockInvalid = errors.New("incoming block is invalid")
	ErrFailedToAddBlock     = errors.New("failed to add block to blockchain")
)

// Validator represents a node that participates in the Proof-of-Stake consensus mechanism.
// For EmPower1, this includes core attributes like Address and Stake, and is extensible
// for future reputation and activity-based weighting.
type Validator struct {
	Address  []byte // Public key bytes or unique cryptographically derived identifier of the validator
	Stake    uint64 // Amount of PTCN tokens staked by the validator (using uint64 for clarity and consistency with amounts)
	IsActive bool   // Whether the validator is currently active in the consensus set (can be toggled by governance/slashing)
	// V2+ Enhancements for EmPower1's unique consensus:
	// ReputationScore float64 // Derived from on-chain behavior, uptime, AI audit flags (0.0 to 1.0)
	// ActivityScore   float64 // Derived from participation in network tasks, dApp usage (e.g., in EmPower1's ecosystem)
	// LastProposedHeight int64 // For round-robin fairness, or last active contribution
	// JailedUntil     int64 // Timestamp if validator is temporarily jailed
	// UnbondingHeight int64 // Height at which stake will be fully unbonded
	// V2+: NodeID      string // Unique network ID, separate from cryptographic address
}

// NewValidator creates a new Validator instance.
// This constructor ensures basic validity, adhering to "Know Your Core, Keep it Clear".
func NewValidator(address []byte, stake uint64) (*Validator, error) {
	if len(address) == 0 {
		return nil, ErrInvalidValidatorAddress
	}
	// In a real blockchain, you'd validate address format (e.g., checksum, length).

	if stake == 0 { // Stake must be positive to participate in PoS
		return nil, ErrInvalidValidatorStake
	}

	return &Validator{
		Address:  address,
		Stake:    stake,
		IsActive: true, // Assume active by default on creation, can be changed later
	}, nil
}

// Equals checks if two Validator instances represent the same entity, based on their Address.
// This is critical for managing validator sets and preventing duplicates.
func (v *Validator) Equals(other *Validator) bool {
	if v == nil || other == nil { // A validator cannot be nil for comparison
		return false
	}
	// Use bytes.Equal for byte slice comparison, which is correct for addresses.
	return bytes.Equal(v.Address, other.Address)
}

// String provides a human-readable string representation of the Validator.
// Useful for logging and debugging, aligning with "Sense the Landscape".
func (v *Validator) String() string {
	status := "Active"
	if !v.IsActive {
		status = "Inactive"
	}
	return fmt.Sprintf("Validator{Address: %x, Stake: %d, Status: %s}", v.Address, v.Stake, status)
}

// --- Conceptual V2+ Methods for EmPower1's Enhanced PoS ---
/*
// UpdateReputation conceptually updates the validator's reputation score.
// This would be called by the consensus engine or a governance pallet.
func (v *Validator) UpdateReputation(scoreChange float64) {
    v.ReputationScore += scoreChange
    if v.ReputationScore > 1.0 { v.ReputationScore = 1.0 }
    if v.ReputationScore < 0.0 { v.ReputationScore = 0.0 }
    // Emit an event for reputation update
}

// UpdateActivityScore conceptually updates the validator's activity score.
func (v *Validator) UpdateActivityScore(activityDelta float64) {
    v.ActivityScore += activityDelta
    // Add decay or max limits
}

// TotalWeight calculates the validator's effective weight in consensus.
// This is where EmPower1's unique hybrid PoS logic resides.
func (v *Validator) TotalWeight() uint64 {
    // Example: (Stake + (Stake * ReputationScore * ActivityScore)) or other complex formula
    // This is the core algorithm for weighted proposer selection/voting.
    weight := v.Stake
    // if v.ReputationScore > 0 && v.ActivityScore > 0 {
    //    weight += uint64(float64(v.Stake) * v.ReputationScore * v.ActivityScore)
    // }
    return weight
}

// IsEligible checks if a validator is currently eligible to propose/validate.
// Considers active status, jailed status, minimum stake etc.
func (v *Validator) IsEligible(currentHeight int64) bool {
    // Example:
    // return v.IsActive && v.Stake >= minValidatorStake && v.JailedUntil <= currentHeight
    return v.IsActive // Basic for now
}
*/

// SimulatedNetwork interface defines methods for network communication in the consensus engine.
type SimulatedNetwork interface {
	BroadcastBlock(block *core.Block) error
	ReceiveBlocks() <-chan *core.Block
}

// ConsensusEngine manages the consensus process, including proposing and validating blocks.
type ConsensusEngine struct {
	validatorAddress  string
	isValidator       bool
	proposerService   *ProposerService
	validationService *ValidationService
	consensusState    *ConsensusState
	blockchain        *core.Blockchain
	network           SimulatedNetwork

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	logger    *log.Logger
	isRunning atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewConsensusEngine creates a new instance of ConsensusEngine with the necessary dependencies.
func NewConsensusEngine(
	validatorAddress string,
	proposerService *ProposerService,
	validationService *ValidationService,
	consensusState *ConsensusState,
	blockchain *core.Blockchain,
	network SimulatedNetwork,
) (*ConsensusEngine, error) {
	if proposerService == nil || validationService == nil || consensusState == nil || blockchain == nil || network == nil {
		return nil, fmt.Errorf("%w: all core services must be provided", ErrInvalidEngineConfig)
	}
	isValidator := (proposerService != nil && proposerService.validatorAddress == validatorAddress)
	logger := log.New(os.Stdout, "CONSENSUS_ENGINE: ", log.Ldate|log.Ltime|log.Lshortfile)
	ctx, cancel := context.WithCancel(context.Background())

	engine := &ConsensusEngine{
		validatorAddress:  validatorAddress,
		isValidator:       isValidator,
		proposerService:   proposerService,
		validationService: validationService,
		consensusState:    consensusState,
		blockchain:        blockchain,
		network:           network,
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
	}
	engine.logger.Println("ConsensusEngine initialized.")
	return engine, nil
}

// Start begins the consensus engine's operation, enabling block proposal and validation.
func (ce *ConsensusEngine) Start() error {
	var err error
	ce.startOnce.Do(func() {
		if ce.isRunning.Load() {
			err = ErrEngineAlreadyRunning
			return
		}
		ce.isRunning.Store(true)
		ce.wg.Add(2)
		go ce.startEngineLoop()
		go ce.processIncomingBlocks()
		ce.logger.Println("ConsensusEngine started.")
	})
	return err
}

// Stop halts the consensus engine, disabling block proposal and validation.
func (ce *ConsensusEngine) Stop() error {
	var err error
	ce.stopOnce.Do(func() {
		if !ce.isRunning.Load() {
			err = ErrEngineNotRunning
			return
		}
		ce.cancel()
		ce.wg.Wait()
		ce.isRunning.Store(false)
		ce.logger.Println("ConsensusEngine stopped.")
	})
	return err
}

func (ce *ConsensusEngine) startEngineLoop() {
	defer ce.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ce.logger.Println("Engine loop started.")

	for {
		select {
		case <-ce.ctx.Done():
			ce.logger.Println("Engine loop received stop signal.")
			return
		case <-ticker.C:
			currentChainHeight := ce.blockchain.ChainHeight()
			if currentChainHeight == -1 {
				ce.logger.Println("Blockchain is empty, waiting for genesis block to sync or be created.")
				continue
			}
			nextBlockHeight := currentChainHeight + 1
			expectedProposer, err := ce.consensusState.GetProposerForHeight(nextBlockHeight)
			if err != nil {
				ce.logger.Printf("Failed to get proposer for height %d: %v", nextBlockHeight, err)
				continue
			}
			if ce.isValidator && bytes.Equal([]byte(ce.validatorAddress), expectedProposer.Address) {
				ce.logger.Printf("It's OUR turn to propose block #%d!", nextBlockHeight)
				if err := ce.proposeBlock(nextBlockHeight); err != nil {
					ce.logger.Printf("Failed to propose block #%d: %v", nextBlockHeight, err)
				}
			}
		}
	}
}

func (ce *ConsensusEngine) proposeBlock(height int64) error {
	lastBlock, err := ce.blockchain.GetLastBlock()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToGetLastBlock, err)
	}
	proposal, err := ce.proposerService.CreateProposalBlock(height, lastBlock.Hash, lastBlock.Timestamp)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProposeBlockFailed, err)
	}
	if err := ce.network.BroadcastBlock(proposal); err != nil {
		return fmt.Errorf("failed to broadcast proposed block %d: %w", height, err)
	}
	ce.logger.Printf("Proposed and broadcasted block #%d (%x).", proposal.Height, proposal.Hash)
	return nil
}

func (ce *ConsensusEngine) processIncomingBlocks() {
	defer ce.wg.Done()
	ce.logger.Println("Incoming block processor started.")

	for {
		select {
		case <-ce.ctx.Done():
			ce.logger.Println("Incoming block processor received stop signal.")
			return
		case incomingBlock := <-ce.network.ReceiveBlocks():
			ce.logger.Printf("Received block #%d (%x) from network. Proposer: %x",
				incomingBlock.Height, incomingBlock.Hash, incomingBlock.ProposerAddress)
			if err := ce.validationService.ValidateBlock(incomingBlock); err != nil {
				ce.logger.Printf("Incoming block #%d (%x) is INVALID: %v", incomingBlock.Height, incomingBlock.Hash, err)
				continue
			}
			if err := ce.blockchain.AddBlock(incomingBlock); err != nil {
				ce.logger.Printf("Failed to add validated block #%d (%x) to blockchain: %v", incomingBlock.Height, incomingBlock.Hash, err)
				continue
			}
			if err := ce.consensusState.UpdateHeight(incomingBlock.Height); err != nil {
				ce.logger.Printf("Failed to update consensus state height after adding block %d: %v", incomingBlock.Height, err)
			}
		}
	}
}
