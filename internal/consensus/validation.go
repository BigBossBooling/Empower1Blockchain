package consensus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"empower1.com/core/core"
)

// Define custom errors specific to ValidationService for clearer failure states.
var (
	ErrValidationServiceInit = errors.New("validation service initialization error")
	ErrNilBlock              = errors.New("cannot validate a nil block")
	ErrBlockIntegrity        = errors.New("block integrity check failed")
	ErrChainContinuity       = errors.New("chain continuity check failed")
	ErrTimeProtocol          = errors.New("time protocol violation")
	ErrProposerMismatch      = errors.New("proposer mismatch")
	ErrSignatureVerification = errors.New("signature verification failed")
	ErrBlockHashMismatch     = errors.New("block hash mismatch")
	ErrTransactionIntegrity  = errors.New("transaction integrity check failed")

	ErrEngineAlreadyRunning = errors.New("consensus engine is already running")
	ErrEngineNotRunning     = errors.New("consensus engine is not running")
	ErrInvalidEngineConfig  = errors.New("invalid consensus engine configuration")
	ErrFailedToGetLastBlock = errors.New("failed to get last block from blockchain")
	ErrFailedToGetProposer  = errors.New("failed to get proposer for height")
	ErrProposeBlockFailed   = errors.New("failed to propose block")
	ErrIncomingBlockInvalid = errors.New("incoming block is invalid")
	ErrFailedToAddBlock     = errors.New("failed to add block to blockchain")
)

// ValidationService is responsible for validating blocks according to PoS consensus rules.
// It acts as the primary gatekeeper for blocks entering the blockchain.
type ValidationService struct {
	consensusState *ConsensusState
	blockchain     *core.Blockchain // Direct dependency on the core blockchain instance
	logger         *log.Logger      // Dedicated logger for the ValidationService
}

// NewValidationService creates a new ValidationService instance.
// It takes dependencies (ConsensusState, Blockchain) to perform its checks.
func NewValidationService(cs *ConsensusState, bc *core.Blockchain) (*ValidationService, error) {
	if cs == nil {
		return nil, fmt.Errorf("%w: ConsensusState cannot be nil", ErrValidationServiceInit)
	}
	if bc == nil {
		return nil, fmt.Errorf("%w: Blockchain cannot be nil", ErrValidationServiceInit)
	}

	logger := log.New(os.Stdout, "VALIDATION: ", log.Ldate|log.Ltime|log.Lshortfile)

	vs := &ValidationService{
		consensusState: cs,
		blockchain:     bc,
		logger:         logger,
	}
	vs.logger.Println("ValidationService initialized.")
	return vs, nil
}

// ValidateBlock performs a comprehensive set of checks on a block to ensure it conforms to consensus rules.
// This is the core function for maintaining blockchain integrity and security.
// It directly supports "Sense the Landscape, Secure the Solution" for EmPower1.
func (vs *ValidationService) ValidateBlock(block *core.Block) error {
	if block == nil {
		return ErrNilBlock
	}

	vs.logger.Printf("VALIDATING: Block #%d (%x) proposed by %x. PrevHash: %x",
		block.Height, block.Hash, block.ProposerAddress, block.PrevBlockHash)

	// --- 1. Basic Structural & Cryptographic Integrity Checks (First Line of Defense) ---
	// These are fundamental checks that can prevent processing malformed blocks.
	if block.Hash == nil || len(block.Hash) != sha256.Size {
		return fmt.Errorf("%w: block hash is missing or has invalid length (%d bytes, expected %d)", ErrBlockIntegrity, len(block.Hash), sha256.Size)
	}
	if block.ProposerAddress == nil || len(block.ProposerAddress) == 0 {
		return fmt.Errorf("%w: block proposer address is missing", ErrBlockIntegrity)
	}
	if block.Signature == nil || len(block.Signature) == 0 {
		return fmt.Errorf("%w: block signature is missing", ErrBlockIntegrity)
	}

	// Re-verify block hash (ensure it matches the content it claims to hash)
	// This is a critical check against block tampering.
	originalHash := block.Hash
	tempBlock := *block  // Create a copy to recalculate hash without modifying original
	tempBlock.Hash = nil // Clear current hash for recalculation
	tempBlock.SetHash()  // Recalculate hash based on received content

	if !bytes.Equal(originalHash, tempBlock.Hash) {
		vs.logger.Printf("SLASHING_EVENT (LOG): Block #%d proposed by %x - Content hash mismatch. Received %x, Calculated %x\n",
			block.Height, block.ProposerAddress, originalHash, tempBlock.Hash)
		return fmt.Errorf("%w: block content (%x) does not match its claimed hash (%x)", ErrBlockHashMismatch, tempBlock.Hash, originalHash)
	}
	// Restore original hash to the block (as we validated it, not changed it)
	block.Hash = originalHash

	// --- 2. Chain Continuity & Time Protocol Checks (Ensuring Law of Constant Progression) ---
	if block.Height > 0 { // For all blocks after genesis
		lastBlock, err := vs.blockchain.GetLastBlock()
		if err != nil {
			// This indicates a severe issue with the local blockchain state if not height 0.
			if errors.Is(err, core.ErrBlockchainEmpty) { // If our chain is unexpectedly empty
				return fmt.Errorf("%w: local blockchain is empty, cannot validate non-genesis block #%d", ErrChainContinuity, block.Height)
			}
			return fmt.Errorf("%w: failed to get last block for chain continuity check: %v", ErrChainContinuity, err)
		}

		if block.Height != lastBlock.Height+1 {
			return fmt.Errorf("%w: invalid block height. Expected %d, got %d (last block %d)", ErrChainContinuity, lastBlock.Height+1, block.Height, lastBlock.Height)
		}
		if !bytes.Equal(block.PrevBlockHash, lastBlock.Hash) {
			return fmt.Errorf("%w: invalid previous block hash. Expected %x, got %x", ErrChainContinuity, lastBlock.Hash, block.PrevBlockHash)
		}

		// Block timestamp must be strictly after previous block's timestamp
		if block.Timestamp <= lastBlock.Timestamp {
			return fmt.Errorf("%w: block timestamp (%d) must be strictly after previous block timestamp (%d)", ErrTimeProtocol, block.Timestamp, lastBlock.Timestamp)
		}
	} else { // Specifically for genesis block (Height == 0)
		if vs.blockchain.ChainHeight() != -1 && vs.blockchain.ChainHeight() != 0 {
			// This means we already have a blockchain, so a new genesis shouldn't be added.
			return fmt.Errorf("%w: blockchain already initialized, cannot accept new genesis block at height 0", ErrChainContinuity)
		}
		// For genesis, PrevBlockHash should be the designated genesis hash (all zeros)
		if !bytes.Equal(block.PrevBlockHash, bytes.Repeat([]byte{0x00}, sha256.Size)) {
			return fmt.Errorf("%w: genesis block PrevBlockHash is incorrect", ErrChainContinuity)
		}
		// Genesis timestamp can be anything in the past, but typical fixed.
	}

	// Check block timestamp relative to current node time (clock drift tolerance)
	// This helps prevent blocks from the far past/future due to clock skew or attacks.
	maxDriftFuture := 10 * time.Second // Max 10s in the future
	maxDriftPast := 5 * time.Minute    // Max 5 min in the past (more lenient for network sync)

	currentTime := time.Now()
	blockTime := time.Unix(0, block.Timestamp)

	if blockTime.After(currentTime.Add(maxDriftFuture)) {
		return fmt.Errorf("%w: block timestamp %s is too far in the future (current time %s)", ErrTimeProtocol, blockTime, currentTime)
	}
	// For non-genesis blocks, ensure it's not excessively old relative to now, assuming active network
	if block.Height > 0 && blockTime.Before(currentTime.Add(-maxDriftPast)) {
		vs.logger.Printf("VALIDATION_WARN: Block #%d timestamp %s is significantly old (current time %s). Possible network latency or old proposal.\n", block.Height, blockTime, currentTime)
		// Depending on network rules, this might be a warning or a soft rejection.
		// For now, it's a warning, but could be an error in stricter production.
	}

	// --- 3. Proposer Legitimacy & Signature Verification ---
	// This ensures the block was proposed by the correct validator and is cryptographically signed.
	expectedProposer, err := vs.consensusState.GetProposerForHeight(block.Height)
	if err != nil {
		return fmt.Errorf("%w: failed to determine expected proposer for height %d: %v", ErrProposerMismatch, block.Height, err)
	}
	// Verify proposer address matches the expected one for this height.
	if !bytes.Equal(block.ProposerAddress, expectedProposer.Address) {
		vs.logger.Printf("SLASHING_EVENT (LOG): Block #%d proposed by %x, expected %x. (Wrong proposer address)\n", block.Height, block.ProposerAddress, expectedProposer.Address)
		return fmt.Errorf("%w: block proposed by wrong validator: expected %x, got %x", ErrProposerMismatch, expectedProposer.Address, block.ProposerAddress)
	}

	// Verify block signature using the block's content
	// This is crucial for ensuring the block was indeed proposed by the holder of the private key.
	err = verifySignature(block.ProposerAddress, block.Signature, block.Hash)
	if err != nil {
		vs.logger.Printf("SLASHING_EVENT (LOG): Block #%d signature verification failed for proposer %x: %v\n", block.Height, block.ProposerAddress, err)
		return fmt.Errorf("%w: block signature verification failed: %v", ErrSignatureVerification, err)
	}

	// --- 4. Transaction Integrity Checks (Ensuring Valid Payload) ---
	// If there are transactions, validate each one according to the network's rules.
	// This is essential to prevent invalid or malicious transactions from being included in the block.
	if len(block.Transactions) > 0 {
		// For now, we just check that each transaction is well-formed.
		// In a real implementation, this would involve checking signatures, amounts, etc.
		for _, tx := range block.Transactions {
			if tx == nil {
				return fmt.Errorf("%w: block contains a nil transaction", ErrTransactionIntegrity)
			}
			// Example: Check that transaction has a valid signature (placeholder, implement actual logic)
			if tx.Signature == nil || len(tx.Signature) == 0 {
				return fmt.Errorf("%w: transaction %x signature is missing", ErrTransactionIntegrity, tx.Hash)
			}
			// TODO: Add more transaction validation logic as per network requirements.
		}
	}

	vs.logger.Printf("VALIDATED: Block #%d (%x) is valid.", block.Height, block.Hash)
	return nil
}

// SimulatedNetwork interface represents the methods required for network communication in the consensus engine.
// This abstraction allows for easier testing and simulation of network conditions.
type SimulatedNetwork interface {
	BroadcastBlock(block *core.Block) error
	ReceiveBlocks() <-chan *core.Block
}

// ConsensusEngine is the core component that drives the consensus algorithm.
// It manages the lifecycle of the consensus process, block proposal, and validation.
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
// It initializes the engine but does not start it.
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

// Start begins the consensus engine's operation.
// It enters the main loop where it proposes and validates blocks according to the consensus algorithm.
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

// Stop gracefully halts the consensus engine's operation.
// It waits for the ongoing processes to complete before shutting down.
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

// startEngineLoop is the main loop for the consensus engine.
// It periodically checks if it's the validator's turn to propose a new block and does so if applicable.
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

// proposeBlock handles the creation and broadcasting of a new block proposal.
// It retrieves the last block, creates a proposal for the next block, and broadcasts it to the network.
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

// processIncomingBlocks listens for and processes incoming blocks from the network.
// It validates each block and adds it to the blockchain if valid.
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
