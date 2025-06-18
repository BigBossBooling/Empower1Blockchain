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

	// Ensure this import path matches your core package structure
	"empower1.com/core/core"
)

// Define custom errors specific to the ProposerService for clearer failure states.
var (
	ErrProposerNotConfigured    = errors.New("proposer service not configured")
	ErrMempoolUnavailable       = errors.New("mempool retriever not configured")
	ErrTransactionSerialization = errors.New("failed to serialize transactions for block data")
	ErrBlockSigningFailed       = errors.New("failed to sign block proposal")
	ErrInvalidBlockTimestamp    = errors.New("invalid block timestamp")
)

// ProposerService is responsible for creating new block proposals when it's the node's turn.
// It gathers transactions, constructs the block, signs it, and calculates its hash.
type ProposerService struct {
	validatorAddress []byte // Public key bytes of the validator (immutable for this instance)
	privateKey       []byte // Corresponding private key bytes (highly sensitive)
	mempool          MempoolRetriever
	logger           *log.Logger // Dedicated logger for proposer service
}

// MempoolRetriever defines the interface for fetching transactions from a mempool.
// This adheres to the dependency inversion principle, making ProposerService testable and modular.
type MempoolRetriever interface {
	GetPendingTransactions(maxCount int) []*core.Transaction
	// Note: Removal of transactions from mempool should happen after block finalization/commitment
	// by a separate service that observes chain state. Proposer only reads.
}

// NewProposerService creates a new ProposerService instance.
// It requires the validator's address (public key bytes) and corresponding private key bytes.
// In a real application, privateKey would be loaded securely (e.g., from a KMS or encrypted storage).
func NewProposerService(validatorAddress []byte, privateKey []byte, mp MempoolRetriever) (*ProposerService, error) {
	if len(validatorAddress) == 0 {
		return nil, fmt.Errorf("%w: validator address cannot be empty", ErrProposerNotConfigured)
	}
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("%w: private key cannot be empty", ErrProposerNotConfigured)
	}
	if mp == nil {
		return nil, fmt.Errorf("%w: mempool retriever cannot be nil", ErrMempoolUnavailable)
	}

	logger := log.New(os.Stdout, "PROPOSER: ", log.Ldate|log.Ltime|log.Lshortfile) // Initialize logger

	ps := &ProposerService{
		validatorAddress: validatorAddress,
		privateKey:       privateKey,
		mempool:          mp,
		logger:           logger,
	}
	ps.logger.Printf("ProposerService initialized for validator: %x", ps.validatorAddress)
	return ps, nil
}

const (
	maxTxsPerBlock       = 100                 // Max transactions to include in a block, aligning with "Systematize for Scalability"
	maxFutureTimeOffset  = 5 * time.Second     // Max allowable future timestamp for a block
	minTimeBetweenBlocks = 1 * time.Nanosecond // Smallest allowed time diff for sequential blocks
)

// CreateProposalBlock creates a new block proposal.
// It orchestrates transaction gathering, block structuring, signing, and hashing.
// This function is central to the PoS consensus "Kinetic System".
func (ps *ProposerService) CreateProposalBlock(height int64, prevBlockHash []byte, prevBlockTimestamp int64) (*core.Block, error) {
	if ps.validatorAddress == nil || len(ps.validatorAddress) == 0 || ps.privateKey == nil || len(ps.privateKey) == 0 {
		return nil, ErrProposerNotConfigured
	}
	if ps.mempool == nil {
		return nil, ErrMempoolUnavailable
	}

	ps.logger.Printf("Creating proposal for block #%d. PrevHash: %x. PrevTimestamp: %d", height, prevBlockHash, prevBlockTimestamp)

	// 1. Gather transactions from the mempool (Input: current blockchain state, Mempool)
	// This ensures our block is useful and contains relevant pending transactions.
	pendingTxs := ps.mempool.GetPendingTransactions(maxTxsPerBlock)

	// 2. Validate transactions (Conceptual: AI/ML pre-validation for EmPower1)
	// This is where AI/ML logic might perform a quick pre-check on transactions
	// before they are even included in a proposed block, enhancing "Sense the Landscape".
	for i, tx := range pendingTxs {
		// Example: Call a conceptual AI/ML module's pre-validation
		// if !ai_ml_module.PreValidateTxForInclusion(tx) {
		// 	ps.logger.Printf("PROPOSER_WARN: Transaction %x rejected by AI pre-validation for block #%d", tx.ID, height)
		// 	pendingTxs = append(pendingTxs[:i], pendingTxs[i+1:]...) // Remove problematic tx
		// }
	}

	// 3. Create the block structure
	block := core.NewBlock(height, prevBlockHash, pendingTxs)

	// 4. Set block timestamp (Crucial for liveness and ordering)
	// Timestamp must be greater than previous block's but not too far in the future.
	currentTime := time.Now().UnixNano()
	if height > 0 && currentTime <= prevBlockTimestamp {
		// If current time is not strictly greater than previous, increment previous timestamp by min allowed.
		// This ensures strict monotonic increase of timestamps for non-genesis blocks.
		block.Timestamp = prevBlockTimestamp + minTimeBetweenBlocks
		ps.logger.Printf("PROPOSER_WARN: Current time %d not greater than prev block timestamp %d. Setting block #%d timestamp to %d (prev + min_diff)", currentTime, prevBlockTimestamp, height, block.Timestamp)
	} else {
		block.Timestamp = currentTime
	}
	// Basic check: prevent proposals with excessively future timestamps.
	if block.Timestamp > time.Now().Add(maxFutureTimeOffset).UnixNano() {
		return nil, fmt.Errorf("%w: block timestamp too far in the future", ErrInvalidBlockTimestamp)
	}

	// 5. Sign the block's header
	// The block's ProposerAddress is set here before signing.
	err := block.Sign(ps.validatorAddress, ps.privateKey)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to sign block proposal for block %d: %v", ErrBlockSigningFailed, height, err)
	}

	// 6. Calculate and set the final hash
	// The hash is calculated AFTER signing and all header/finalization fields are set.
	block.SetHash()

	ps.logger.Printf("ProposerService: Created proposal block #%d (%x) by %x with %d txs. PrevHash: %x\n",
		block.Height, block.Hash, ps.validatorAddress, len(pendingTxs), block.PrevBlockHash)
	return block, nil
}

// Note: The actual removal of transactions from the mempool after they are included in a block
// should be handled by the component that confirms block finality and updates the blockchain state
// (e.g., the `Chain` or `Consensus` manager that commits blocks). The ProposerService only reads from the mempool.

// Define custom errors for the consensus engine
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

// SimulatedNetwork interface for network operations in the consensus engine
type SimulatedNetwork interface {
	BroadcastBlock(block *core.Block) error
	ReceiveBlocks() <-chan *core.Block
}

// ConsensusEngine orchestrates the consensus process, including proposing and validating blocks.
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

// Start begins the consensus engine's operation, entering the main loop for proposing and validating blocks.
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

// Stop halts the consensus engine, stopping all operations and waiting for the goroutines to finish.
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

// startEngineLoop is the main loop of the consensus engine, responsible for proposing new blocks.
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

// proposeBlock handles the proposal of a new block, including its creation and broadcasting.
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
