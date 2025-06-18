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

// --- Custom Errors for ConsensusEngine ---
var (
    ErrEngineAlreadyRunning  = errors.New("consensus engine is already running")
    ErrEngineNotRunning      = errors.New("consensus engine is not running")
    ErrInvalidEngineConfig   = errors.New("invalid consensus engine configuration")
    ErrFailedToGetLastBlock  = errors.New("failed to retrieve last block from blockchain")
    ErrFailedToGetProposer   = errors.New("failed to determine proposer for height")
    ErrProposeBlockFailed    = errors.New("block proposal creation or signing failed")
    ErrIncomingBlockInvalid  = errors.New("incoming block failed validation")
    ErrFailedToAddBlock      = errors.New("failed to add validated block to blockchain")
    ErrConsensusStateUpdate  = errors.New("failed to update consensus state height")
    ErrBroadcastFailed       = errors.New("failed to broadcast block")
)

// SimulatedNetwork defines the interface for conceptual network interactions.
type SimulatedNetwork interface {
    BroadcastBlock(block *core.Block) error
    ReceiveBlocks() <-chan *core.Block
}

// ConsensusEngine orchestrates the PoS consensus process for EmPower1 Blockchain.
type ConsensusEngine struct {
    validatorAddress  []byte
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

// NewConsensusEngine creates a new ConsensusEngine instance.
func NewConsensusEngine(
    validatorAddress []byte,
    proposerService *ProposerService,
    validationService *ValidationService,
    consensusState *ConsensusState,
    blockchain *core.Blockchain,
    network SimulatedNetwork,
) (*ConsensusEngine, error) {
    if proposerService == nil || validationService == nil || consensusState == nil || blockchain == nil || network == nil {
        return nil, fmt.Errorf("%w: all core services (proposer, validator, state, blockchain, network) must be provided", ErrInvalidEngineConfig)
    }
    if len(validatorAddress) == 0 {
        return nil, fmt.Errorf("%w: validator address for this node cannot be empty", ErrInvalidEngineConfig)
    }
    isValidator := bytes.Equal(proposerService.validatorAddress, validatorAddress)
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

// Start initiates the consensus engine's operation.
func (ce *ConsensusEngine) Start() error {
    var err error
    ce.startOnce.Do(func() {
        if ce.isRunning.Load() {
            err = ErrEngineAlreadyRunning
            return
        }
        ce.isRunning.Store(true)
        ce.wg.Add(1)
        go ce.engineLoop()
        ce.wg.Add(1)
        go ce.handleIncomingBlocks()
        ce.logger.Println("ConsensusEngine started successfully.")
    })
    return err
}

// Stop gracefully shuts down the consensus engine.
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
        ce.logger.Println("ConsensusEngine stopped gracefully.")
    })
    return err
}

// engineLoop is the main loop for the consensus engine.
func (ce *ConsensusEngine) engineLoop() {
    defer ce.wg.Done()
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    ce.logger.Println("Consensus engine proposal loop started.")

    for {
        select {
        case <-ce.ctx.Done():
            ce.logger.Println("Consensus engine proposal loop received stop signal.")
            return
        case <-ticker.C:
            currentChainHeight := ce.blockchain.ChainHeight()
            if currentChainHeight == -1 {
                ce.logger.Println("Blockchain is empty; proposal loop waiting for genesis block to sync or be created locally.")
                continue
            }
            nextBlockHeight := currentChainHeight + 1
            expectedProposer, err := ce.consensusState.GetProposerForHeight(nextBlockHeight)
            if err != nil {
                ce.logger.Printf("Failed to determine expected proposer for height %d: %v", nextBlockHeight, err)
                continue
            }
            if ce.isValidator && bytes.Equal(ce.validatorAddress, expectedProposer.Address) {
                ce.logger.Printf("It's OUR turn to propose block #%d! Proposer: %x", nextBlockHeight, ce.validatorAddress)
                if err := ce.proposeBlock(nextBlockHeight); err != nil {
                    ce.logger.Printf("Failed to propose block #%d: %v", nextBlockHeight, err)
                }
            }
        }
    }
}

// proposeBlock orchestrates the creation, signing, and broadcasting of a new block proposal.
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
        return fmt.Errorf("%w: failed to broadcast proposed block %d (%x): %v", ErrBroadcastFailed, height, proposal.Hash, err)
    }
    ce.logger.Printf("Proposed and broadcasted block #%d (%x) by %x. PrevHash: %x.", proposal.Height, proposal.Hash, proposal.ProposerAddress, proposal.PrevBlockHash)
    return nil
}

// handleIncomingBlocks listens for blocks received from the network and orchestrates their validation and addition.
func (ce *ConsensusEngine) handleIncomingBlocks() {
    defer ce.wg.Done()
    ce.logger.Println("Incoming block processor started.")

    for {
        select {
        case <-ce.ctx.Done():
            ce.logger.Println("Incoming block processor received stop signal.")
            return
        case incomingBlock := <-ce.network.ReceiveBlocks():
            if incomingBlock == nil {
                ce.logger.Println("Received nil block from network. Skipping processing.")
                continue
            }
            ce.logger.Printf("Received block #%d (%x) from network. Proposer: %x. PrevHash: %x",
                incomingBlock.Height, incomingBlock.Hash, incomingBlock.ProposerAddress, incomingBlock.PrevBlockHash)
            if err := ce.validationService.ValidateBlock(incomingBlock); err != nil {
                ce.logger.Printf("Incoming block #%d (%x) is INVALID. Proposer: %x. Error: %v. Skipping add.",
                    incomingBlock.Height, incomingBlock.Hash, incomingBlock.ProposerAddress, err)
                continue
            }
            if err := ce.blockchain.AddBlock(incomingBlock); err != nil {
                ce.logger.Printf("Failed to add VALIDATED block #%d (%x) to blockchain: %v", incomingBlock.Height, incomingBlock.Hash, err)
                continue
            }
            if err := ce.consensusState.UpdateHeight(incomingBlock.Height); err != nil {
                ce.logger.Printf("Failed to update consensus state height after adding block %d (%x): %v", incomingBlock.Height, incomingBlock.Hash, err)
            }
            ce.logger.Printf("Successfully processed block #%d (%x). ChainHeight: %d. ConsensusState Height: %d.",
                incomingBlock.Height, incomingBlock.Hash, ce.blockchain.ChainHeight(), ce.consensusState.CurrentHeight())
        }
    }
}