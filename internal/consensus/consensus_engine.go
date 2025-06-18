package consensus

import (
	"bytes"
	"empower1.com/core/core" // Assuming 'core' is the package alias for empower1.com/core/core
	"errors"
	"fmt"
	"log" // For structured logging
	"os"   // For log output
	"time"
)

// --- Custom Errors for ConsensusEngine ---
var (
	ErrEngineAlreadyRunning  = errors.New("consensus engine is already running")
	ErrEngineNotRunning      = errors.New("consensus engine is not running")
	ErrInvalidEngineConfig   = errors.New("invalid consensus engine configuration")
	ErrFailedToGetLastBlock  = errors.New("failed to get last block from blockchain")
	ErrFailedToGetProposer   = errors.New("failed to get proposer for height")
	ErrProposeBlockFailed    = errors.New("failed to propose block")
	ErrIncomingBlockInvalid  = errors.New("incoming block is invalid")
	ErrFailedToAddBlock      = errors.New("failed to add block to blockchain")
)

// SimulatedNetwork defines an interface for conceptual network interactions.
// In a real blockchain, this would be the actual P2P network layer.
type SimulatedNetwork interface {
	BroadcastBlock(block *core.Block) error
	ReceiveBlocks() <-chan *core.Block // Channel for incoming blocks
}

// ConsensusEngine orchestrates the PoS consensus process.
// It manages block proposal, validation, and chain synchronization.
type ConsensusEngine struct {
	validatorAddress string            // Address of *this* node's validator (empty if not a validator)
	isValidator      bool              // True if this node is configured as a validator
	proposerService  *ProposerService  // Service to create new blocks
	validationService *ValidationService // Service to validate incoming blocks
	consensusState   *ConsensusState   // Current state of consensus (height, validator set, schedule)
	blockchain       *core.Blockchain  // The main blockchain instance
	network          SimulatedNetwork  // Conceptual network interface
	
	stopChan         chan struct{}     // Channel to signal engine shutdown
	isRunning        bool              // Flag to track engine's running status
	wg               sync.WaitGroup    // WaitGroup for goroutines
	logger           *log.Logger       // Dedicated logger for the engine
}

// NewConsensusEngine creates a new ConsensusEngine instance.
// It initializes all core components required for consensus operation.
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
	
	// Determine if this node is a validator based on if a proposer service is provided for its address.
	// In a real system, it would check if validatorAddress is in the active validator set.
	isValidator := (proposerService != nil && proposerService.validatorAddress == validatorAddress) 
	
	logger := log.New(os.Stdout, "CONSENSUS_ENGINE: ", log.Ldate|log.Ltime|log.Lshortfile)

	engine := &ConsensusEngine{
		validatorAddress:  validatorAddress,
		isValidator:       isValidator,
		proposerService:   proposerService,
		validationService: validationService,
		consensusState:    consensusState,
		blockchain:        blockchain,
		network:           network,
		logger:            logger,
		stopChan:          make(chan struct{}),
		isRunning:         false,
	}
	engine.logger.Println("ConsensusEngine initialized.")
	return engine, nil
}

// Start initiates the consensus engine's operation.
// It starts goroutines for block proposal and incoming block processing.
func (ce *ConsensusEngine) Start() error {
	ce.wg.Add(1) // Add for StartEngineLoop
	go ce.startEngineLoop()

	// In a real application, you'd also start goroutines to listen for P2P messages (blocks, transactions, votes).
	// For this conceptual stage, we'll assume `network.ReceiveBlocks()` represents that.
	ce.wg.Add(1) // Add for ProcessIncomingBlocks
	go ce.processIncomingBlocks()

	ce.isRunning = true
	ce.logger.Println("ConsensusEngine started.")
	return nil
}

// Stop gracefully shuts down the consensus engine.
func (ce *ConsensusEngine) Stop() error {
	if !ce.isRunning {
		return ErrEngineNotRunning
	}
	close(ce.stopChan) // Signal goroutines to stop
	ce.wg.Wait()       // Wait for all goroutines to finish
	ce.isRunning = false
	ce.logger.Println("ConsensusEngine stopped.")
	return nil
}

// startEngineLoop is the main loop for the consensus engine.
// It continuously checks if it's this node's turn to propose a block.
func (ce *ConsensusEngine) startEngineLoop() {
	defer ce.wg.Done() // Ensure WaitGroup counter is decremented on exit
	ticker := time.NewTicker(1 * time.Second) // Check every second if it's proposer turn
	defer ticker.Stop()

	ce.logger.Println("Engine loop started.")

	for {
		select {
		case <-ce.stopChan:
			ce.logger.Println("Engine loop received stop signal.")
			return
		case <-ticker.C:
			// Ensure ConsensusState is updated with latest chain height
			currentChainHeight := ce.blockchain.ChainHeight()
			if currentChainHeight == -1 {
				ce.logger.Debug("Blockchain is empty, waiting for genesis block to sync or be created.")
				continue // Cannot propose without a chain
			}

			// Propose for the NEXT block height
			nextBlockHeight := currentChainHeight + 1 
			
			// Get expected proposer for the next height
			expectedProposer, err := ce.consensusState.GetProposerForHeight(nextBlockHeight)
			if err != nil {
				ce.logger.Errorf("Failed to get proposer for height %d: %v", nextBlockHeight, err)
				continue
			}

			// Check if it's this node's turn to propose
			if ce.isValidator && bytes.Equal([]byte(ce.validatorAddress), expectedProposer.Address) { // Address is []byte now
				ce.logger.Printf("It's OUR turn to propose block #%d!", nextBlockHeight)
				if err := ce.proposeBlock(nextBlockHeight); err != nil {
					ce.logger.Errorf("Failed to propose block #%d: %v", nextBlockHeight, err)
				}
			}
		}
	}
}

// proposeBlock orchestrates the creation, signing, and broadcasting of a new block.
func (ce *ConsensusEngine) proposeBlock(height int64) error {
	lastBlock, err := ce.blockchain.GetLastBlock()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToGetLastBlock, err)
	}

	proposal, err := ce.proposerService.CreateProposalBlock(height, lastBlock.Hash, lastBlock.Timestamp)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProposeBlockFailed, err)
	}

	// In a real system, you would then broadcast this proposed block to the network.
	if err := ce.network.BroadcastBlock(proposal); err != nil {
		return fmt.Errorf("failed to broadcast proposed block %d: %w", height, err)
	}
	ce.logger.Printf("Proposed and broadcasted block #%d (%x).", proposal.Height, proposal.Hash)
	return nil
}

// processIncomingBlocks listens for blocks received from the network and validates/adds them.
func (ce *ConsensusEngine) processIncomingBlocks() {
	defer ce.wg.Done()
	ce.logger.Println("Incoming block processor started.")

	for {
		select {
		case <-ce.stopChan:
			ce.logger.Println("Incoming block processor received stop signal.")
			return
		case incomingBlock := <-ce.network.ReceiveBlocks(): // Receive block from conceptual network
			ce.logger.Printf("Received block #%d (%x) from network. Proposer: %x", 
				incomingBlock.Height, incomingBlock.Hash, incomingBlock.ProposerAddress)

			// Validate the incoming block
			if err := ce.validationService.ValidateBlock(incomingBlock); err != nil {
				ce.logger.Errorf("Incoming block #%d (%x) is INVALID: %v", incomingBlock.Height, incomingBlock.Hash, err)
				// In a real system, here we might:
				// - Report the invalid block to a slashing mechanism.
				// - Request other peers for a valid block at this height.
				continue // Skip adding invalid block
			}

			// Add the validated block to our local blockchain
			if err := ce.blockchain.AddBlock(incomingBlock); err != nil {
				ce.logger.Errorf("Failed to add validated block #%d (%x) to blockchain: %v", incomingBlock.Height, incomingBlock.Hash, err)
				// This could indicate a fork, or a block re-org, or a local state issue.
				// Advanced chains handle this with fork choice rules and re-org logic.
				continue
			}

			// Update consensus state with the new block (critical for height progression and proposer scheduling)
			if err := ce.consensusState.UpdateHeight(incomingBlock.Height); err != nil {
				// This should ideally not fail for a valid block, but defensive.
				ce.logger.Errorf("Failed to update consensus state height after adding block %d: %v", incomingBlock.Height, err)
			}
		}
	}
}

// --- Debug / Helper Functions for ConsensusEngine (Optional) ---
/*
func (ce *ConsensusEngine) DebugPrintState() {
	ce.logger.Printf("--- Consensus Engine State ---")
	ce.logger.Printf("  Running: %t", ce.isRunning)
	ce.logger.Printf("  Validator Address: %x", []byte(ce.validatorAddress))
	ce.logger.Printf("  Current Chain Height: %d", ce.blockchain.ChainHeight())
	ce.logger.Printf("  Consensus State Height: %d", ce.consensusState.CurrentHeight())
	ce.logger.Printf("  Validator Set Count: %d", len(ce.consensusState.GetValidatorSet()))
	ce.logger.Printf("------------------------------")
}
*/