package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"empower1.com/core/core"
)

// Define custom errors for the consensus engine, ensuring clear failure states.
var (
	ErrEngineAlreadyRunning = errors.New("consensus engine is already running")
	ErrEngineNotRunning     = errors.New("consensus engine is not running")
	ErrInvalidEngineConfig  = errors.New("invalid consensus engine configuration")
	ErrFailedToGetLastBlock = errors.New("failed to get last block from blockchain")
	ErrFailedToGetProposer  = errors.New("failed to get proposer for height")
	ErrProposeBlockFailed   = errors.New("failed to propose block")
	ErrIncomingBlockInvalid = errors.New("incoming block is invalid")
	ErrFailedToAddBlock     = errors.New("failed to add block to blockchain")
)

// SimulatedNetwork interface represents the network layer for block propagation.
type SimulatedNetwork interface {
	BroadcastBlock(block *core.Block) error
	ReceiveBlocks() <-chan *core.Block
}

// ConsensusEngine is the core struct managing the consensus process.
// It orchestrates block proposal, validation, and state updates.
type ConsensusEngine struct {
	validatorAddress  string             // Address of the validator node
	isValidator       bool               // Flag indicating if this instance is a validator
	proposerService   *ProposerService   // Service responsible for proposing new blocks
	validationService *ValidationService // Service responsible for validating blocks
	consensusState    *ConsensusState    // Current state of the consensus process
	blockchain        *core.Blockchain   // Reference to the blockchain
	network           SimulatedNetwork   // Network interface for block broadcasting and receiving

	ctx       context.Context    // Context for managing lifecycle
	cancel    context.CancelFunc // Cancel function to stop the context
	wg        sync.WaitGroup     // WaitGroup to manage goroutine lifecycles
	logger    *log.Logger        // Logger for the consensus engine
	isRunning atomic.Bool        // Atomic flag indicating if the engine is running
	startOnce sync.Once          // Ensures Start() logic is only executed once
	stopOnce  sync.Once          // Ensures Stop() logic is only executed once
}

// NewConsensusEngine creates a new instance of ConsensusEngine with the provided services and blockchain reference.
// It initializes the logger and context, and checks if the provided configuration is valid.
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

// Start begins the consensus engine's operation, launching the engine loop and block processing goroutines.
// It ensures that the engine is not already running before starting.
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

// Stop gracefully shuts down the consensus engine, stopping the engine loop and block processing goroutines.
// It ensures that the engine is currently running before stopping.
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

// startEngineLoop is the main loop for the consensus engine, running at regular intervals.
// It checks if it's the validator's turn to propose a new block and triggers the block proposal process if so.
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

// proposeBlock handles the block proposal process for the validator.
// It creates a new proposal block based on the last block in the chain and broadcasts it to the network.
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

// processIncomingBlocks handles the reception and processing of incoming blocks from the network.
// It validates and adds the blocks to the blockchain, and updates the consensus state.
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
