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

type SimulatedNetwork interface {
	BroadcastBlock(block *core.Block) error
	ReceiveBlocks() <-chan *core.Block
}

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
