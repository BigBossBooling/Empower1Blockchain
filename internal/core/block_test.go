package consensus

import (
	"bytes"
	"context" // For context.WithCancel, context.Background
	"crypto/sha256"
	"errors"
	"log"    // For structured logging in tests
	"os"     // For log.New output
	"sync"   // For sync.WaitGroup, sync.Mutex for mocks
	"sync/atomic" // For atomic.Bool
	"testing"
	"time"

	"empower1.com/core/core" // Core blockchain entities (Block, Transaction, Validator)
)

// --- Mock Implementations (Enhanced for Realism and Test Coverage) ---
// These mocks meticulously simulate external dependencies, allowing isolated and precise testing of ConsensusEngine.

// mockNetwork simulates the P2P network layer for testing.
// It allows control over blocks/transactions sent and received, and tracks broadcasts.
type mockNetwork struct {
	broadcastedBlocks []*core.Block
	broadcastedTxs    []*core.Transaction
	recvChan          chan *core.Block // Channel for blocks incoming to engine
	txRecvChan        chan *core.Transaction // Channel for transactions incoming to mempool
	mu                sync.Mutex             // Protects broadcasted lists
}

func newMockNetwork() *mockNetwork {
	return &mockNetwork{
		broadcastedBlocks: make([]*core.Block, 0),
		broadcastedTxs:    make([]*core.Transaction, 0),
		recvChan:          make(chan *core.Block, 10), // Buffered for multiple sends/receives
		txRecvChan:        make(chan *core.Transaction, 10),
	}
}

func (m *mockNetwork) BroadcastBlock(block *core.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastedBlocks = append(m.broadcastedBlocks, block)
	return nil
}

func (m *mockNetwork) BroadcastTransaction(tx *core.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastedTxs = append(m.broadcastedTxs, tx)
	return nil
}

func (m *mockNetwork) ReceiveBlocks() <-chan *core.Block {
	return m.recvChan
}

func (m *mockNetwork) ReceiveTransactions() <-chan *core.Transaction {
	return m.txRecvChan
}

func (m *mockNetwork) GetBroadcastedBlocks() []*core.Block {
	m.mu.Lock()
	defer m.mu.Unlock()
	blocksCopy := make([]*core.Block, len(m.broadcastedBlocks))
	copy(blocksCopy, m.broadcastedBlocks)
	return blocksCopy
}

// mockProposerService simulates the ProposerService for testing block creation.
type mockProposerService struct {
	validatorAddress []byte
	proposalErr      error // Error to return on CreateProposalBlock
	nextProposal     *core.Block // Specific block to return next, if set
	proposalCounter  int         // To track how many times CreateProposalBlock is called
}

func newMockProposerService(validatorAddr []byte) *mockProposerService {
	return &mockProposerService{
		validatorAddress: validatorAddr,
	}
}

func (m *mockProposerService) CreateProposalBlock(height int64, prevHash []byte, prevTime int64) (*core.Block, error) {
	m.proposalCounter++
	if m.proposalErr != nil {
		return nil, m.proposalErr
	}
	if m.nextProposal != nil {
		// Return a deep copy to prevent external modification affecting subsequent calls
		p := *m.nextProposal
		return &p, nil
	}
	// Default mock block for success cases, ensuring it's a valid core.Block
	txs := []core.Transaction{} // Can be empty for simplicity in mock
	block := core.NewBlock(height, prevHash, txs)
	block.Timestamp = time.Now().UnixNano() // Use real timestamp for uniqueness
	block.ProposerAddress = m.validatorAddress
	block.Sign(m.validatorAddress, []byte("dummy_privkey")) // Use the actual Sign method
	block.SetHash() // Calculate hash
	return block, nil
}

// mockValidationService simulates the ValidationService.
type mockValidationService struct {
	validateErr error // Error to return on ValidateBlock
}

func newMockValidationService() *mockValidationService {
	return &mockValidationService{}
}
func (m *mockValidationService) ValidateBlock(block *core.Block) error {
	return m.validateErr
}

// mockConsensusState simulates the ConsensusState.
type mockConsensusState struct {
	mu           sync.Mutex // Protects height and nextProposer
	height       int64
	nextProposer *core.Validator
	proposerErr  error // Error to return on GetProposerForHeight
}

func newMockConsensusState(initialHeight int64, nextProposerAddr []byte) *mockConsensusState {
	return &mockConsensusState{
		height:       initialHeight,
		nextProposer: &core.Validator{Address: nextProposerAddr, Stake: 100}, // Dummy validator
	}
}

func (m *mockConsensusState) GetProposerForHeight(height int64) (*core.Validator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proposerErr != nil {
		return nil, m.proposerErr
	}
	return m.nextProposer, nil
}
func (m *mockConsensusState) UpdateHeight(height int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if height <= m.height {
		return errors.New("height not increasing (simulated error)") // Simulate specific error
	}
	m.height = height
	return nil
}
func (m *mockConsensusState) CurrentHeight() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.height
}
func (m *mockConsensusState) GetValidatorSet() []*core.Validator {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to prevent external modification during tests
	setCopy := make([]*core.Validator, len(m.validatorSet))
	copy(setCopy, m.validatorSet)
	return setCopy
}

// mockBlockchain simulates the Blockchain.
type mockBlockchain struct {
	mu           sync.Mutex // Protects height, blocks, lastBlockHash
	height       int64
	blocks       map[string]*core.Block // Store blocks by hash
	lastBlockHash []byte
	addBlockErr  error // Error to return on AddBlock
}

func newMockBlockchain(initialHeight int64) *mockBlockchain {
	// Create a dummy genesis block consistent with core.NewBlockchain's genesis
	genesisTxs := []core.Transaction{core.Transaction{ID: []byte("mock_genesis_tx")}}
	genesis := core.NewBlock(0, bytes.Repeat([]byte{0x00}, sha256.Size), genesisTxs)
	genesis.ProposerAddress = []byte("genesis_proposer")
	genesis.Sign([]byte("genesis_proposer"), []byte("genesis_privkey")) // Use actual Sign
	genesis.SetHash()

	blocksMap := make(map[string]*core.Block)
	blocksMap[string(genesis.Hash)] = genesis

	return &mockBlockchain{
		height:        initialHeight,
		blocks:        blocksMap,
		lastBlockHash: genesis.Hash,
	}
}

func (m *mockBlockchain) AddBlock(block *core.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.addBlockErr != nil {
		return m.addBlockErr
	}
	// Basic validation to simulate chain continuity
	if block.Height != m.height+1 && !(m.height == -1 && block.Height == 0) { // Allow height 0 if empty
		return errors.New("mock: invalid block height for add")
	}
	if !bytes.Equal(block.PrevBlockHash, m.lastBlockHash) && !(m.height == -1 && block.Height == 0) {
		return errors.New("mock: invalid prev block hash for add")
	}

	m.blocks[string(block.Hash)] = block
	m.height = block.Height
	m.lastBlockHash = block.Hash
	log.Printf("MOCK_BLOCKCHAIN: Added block #%d (%x). New height: %d", block.Height, block.Hash, m.height)
	return nil
}

func (m *mockBlockchain) GetLastBlock() (*core.Block, error) {
	m.mu.Lock() // Using Lock for write access to m.height in tests
	defer m.mu.Unlock()
	if m.height == -1 {
		return nil, errors.New("mock: blockchain is empty")
	}
	// Return a deep copy to prevent tests from accidentally modifying the mock's state
	lastBlock := m.blocks[string(m.lastBlockHash)]
	bCopy := *lastBlock
	return &bCopy, nil
}
func (m *mockBlockchain) ChainHeight() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.height
}
func (m *mockBlockchain) GetBlockByHash(hash []byte) (*core.Block, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blocks[string(hash)]
	if !ok {
		return nil, errors.New("mock: block not found by hash")
	}
	// Return a deep copy
	bCopy := *b
	return &bCopy, nil
}
func (m *mockBlockchain) GetBlockByHeight(height int64) (*core.Block, error) { // Add this for completeness of mock
    m.mu.Lock()
    defer m.mu.Unlock()
    for _, block := range m.blocks {
        if block.Height == height {
            bCopy := *block
            return &bCopy, nil
        }
    }
    return nil, errors.New("mock: block not found by height")
}


// --- Test Setup Helper ---

// setupConsensusEngineTest creates a clean set of mock dependencies and a ConsensusEngine.
// This streamlines test setups and ensures consistency.
func setupConsensusEngineTest(t *testing.T, validatorAddr []byte, initialChainHeight int64) (*ConsensusEngine, *mockNetwork, *mockProposerService, *mockValidationService, *mockConsensusState, *mockBlockchain) {
	// Suppress logging during tests to avoid clutter
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stdout) // Ensure logging is re-enabled after test run

	mockNet := newMockNetwork()
	mockProposer := newMockProposerService(validatorAddr)
	mockValidator := newMockValidationService()
	// Pass the actual validator address for the state mock as well, for consistency
	mockState := newMockConsensusState(initialChainHeight, validatorAddr) 
	mockBlockchain := newMockBlockchain(initialChainHeight)

	engine, err := NewConsensusEngine(validatorAddr, mockProposer, mockValidator, mockState, mockBlockchain, mockNet)
	if err != nil {
		t.Fatalf("Engine initialization failed: %v", err)
	}
	return engine, mockNet, mockProposer, mockValidator, mockState, mockBlockchain
}

// --- Tests ---

func TestConsensusEngine_ProposeBlock_Success(t *testing.T) {
	validatorAddr := []byte("validator1_pubkey")
	engine, mockNet, mockProposer, _, _, _ := setupConsensusEngineTest(t, validatorAddr, 0)

	// Start engine and ensure it can stop
	if err := engine.Start(); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}
	defer engine.Stop()

	// Signal expected proposal (polling mockProposer.proposalCounter)
	proposalCalled := make(chan struct{})
	go func() {
		defer close(proposalCalled)
		for {
			select {
			case <-engine.ctx.Done(): return
			case <-time.After(50 * time.Millisecond):
				if mockProposer.proposalCounter > 0 {
					return // ProposeBlock was called
				}
			}
		}
	}()

	select {
	case <-proposalCalled: // Wait for the signal that proposeBlock ran
		log.Println("Test received proposal signal.") // Debug log
	case <-time.After(2 * time.Second): // Give enough time for ticker to tick and goroutine to run
		t.Fatal("Timeout waiting for block proposal.")
	}

	// Assertions for successful proposal
	broadcastedBlocks := mockNet.GetBroadcastedBlocks()
	if len(broadcastedBlocks) == 0 {
		t.Error("Expected block to be broadcasted, got none.")
	}
	if mockProposer.proposalCounter == 0 {
		t.Error("Expected proposerService.CreateProposalBlock to be called, was not.")
	}
	// Further check: broadcasted block should be height 1
	if len(broadcastedBlocks) > 0 && broadcastedBlocks[0].Height != 1 {
		t.Errorf("Broadcasted block height was %d, expected 1", broadcastedBlocks[0].Height)
	}
}

func TestConsensusEngine_HandleIncomingBlock_Valid(t *testing.T) {
	validatorAddr := []byte("validator_node_A")
	otherValidatorAddr := []byte("validator_node_B") // Proposer of incoming block
	engine, mockNet, _, _, mockState, mockBlockchain := setupConsensusEngineTest(t, validatorAddr, 0)

	if err := engine.Start(); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}
	defer engine.Stop()

	// Simulate an incoming valid block from another peer (height 1)
	txs := []core.Transaction{core.Transaction{ID: []byte("tx_inc_block1")}}
	incomingValidBlock := core.NewBlock(1, mockBlockchain.lastBlockHash, txs)
	incomingValidBlock.Timestamp = time.Now().UnixNano() + 10
	incomingValidBlock.ProposerAddress = otherValidatorAddr
	incomingValidBlock.Sign(otherValidatorAddr, []byte("dummy_privkey_other"))
	incomingValidBlock.SetHash()

	// Send the block to the engine's incoming channel
	mockNet.recvChan <- incomingValidBlock

	// Give time for the goroutine to process the block and update state
	time.Sleep(200 * time.Millisecond)

	// Assertions
	if mockBlockchain.ChainHeight() != 1 {
		t.Errorf("Expected blockchain height to be 1, got %d", mockBlockchain.ChainHeight())
	}
	if mockState.CurrentHeight() != 1 {
		t.Errorf("Expected consensus state height to be 1, got %d", mockState.CurrentHeight())
	}
	_, err := mockBlockchain.GetBlockByHash(incomingValidBlock.Hash)
	if err != nil {
		t.Errorf("Expected incoming valid block to be added to blockchain, got error: %v", err)
	}
}

func TestConsensusEngine_HandleIncomingBlock_Invalid(t *testing.T) {
	validatorAddr := []byte("validator_node_C")
	invalidProposerAddr := []byte("invalid_proposer_X")
	engine, mockNet, _, mockValidator, mockState, mockBlockchain := setupConsensusEngineTest(t, validatorAddr, 0)
	mockValidator.validateErr = errors.New("simulated invalid block error") // Mock validation to fail

	if err := engine.Start(); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}
	defer engine.Stop()

	// Simulate an incoming invalid block
	txs := []core.Transaction{core.Transaction{ID: []byte("tx_inc_invalid1")}}
	incomingInvalidBlock := core.NewBlock(1, mockBlockchain.lastBlockHash, txs)
	incomingInvalidBlock.Timestamp = time.Now().UnixNano() + 10
	incomingInvalidBlock.ProposerAddress = invalidProposerAddr
	incomingInvalidBlock.Sign(invalidProposerAddr, []byte("dummy_privkey_invalid"))
	incomingInvalidBlock.SetHash()

	// Send the block to the engine's incoming channel
	mockNet.recvChan <- incomingInvalidBlock

	// Give time for the goroutine to process the block
	time.Sleep(200 * time.Millisecond)

	// Assertions: Chain height and state height should NOT change for invalid block
	if mockBlockchain.ChainHeight() != 0 {
		t.Errorf("Expected blockchain height to remain 0 for invalid block, got %d", mockBlockchain.ChainHeight())
	}
	if mockState.CurrentHeight() != 0 {
		t.Errorf("Expected consensus state height to remain 0 for invalid block, got %d", mockState.CurrentHeight())
	}
	_, err := mockBlockchain.GetBlockByHash(incomingInvalidBlock.Hash)
	if err == nil {
		t.Errorf("Expected invalid block NOT to be added, but it was found")
	}
}

func TestConsensusEngine_StartStop(t *testing.T) {
	validatorAddr := []byte("validator_start_stop")
	engine, _, _, _, _, _ := setupConsensusEngineTest(t, validatorAddr, 0) // Simplified setup

	// Test Start
	if err := engine.Start(); err != nil {
		t.Fatalf("Engine Start() failed: %v", err)
	}
	if !engine.isRunning.Load() { // Check atomic flag
		t.Errorf("Engine.isRunning is false after Start()")
	}
	// Try starting again (should return error)
	err := engine.Start()
	if err == nil || !errors.Is(err, ErrEngineAlreadyRunning) {
		t.Errorf("Expected ErrEngineAlreadyRunning on second Start(), got %v", err)
	}

	// Test Stop
	if err := engine.Stop(); err != nil {
		t.Fatalf("Engine Stop() failed: %v", err)
	}
	if engine.isRunning.Load() { // Check atomic flag
		t.Errorf("Engine.isRunning is true after Stop()")
	}
	// Try stopping again (should return error)
	err = engine.Stop()
	if err == nil || !errors.Is(err, ErrEngineNotRunning) {
		t.Errorf("Expected ErrEngineNotRunning on second Stop(), got %v", err)
	}
}

// TestConsensusEngine_ProposeBlock_Failure simulates proposal failure and checks logging.
func TestConsensusEngine_ProposeBlock_Failure(t *testing.T) {
	validatorAddr := []byte("validator_fail_propose")
	engine, mockNet, mockProposer, _, _, _ := setupConsensusEngineTest(t, validatorAddr, 0)
	mockProposer.proposalErr = errors.New("simulated proposer error") // Mock proposer to fail

	if err := engine.Start(); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}
	defer engine.Stop()

	// Wait for the engineLoop to tick and call proposeBlock
	time.Sleep(1500 * time.Millisecond) // Give enough time for the ticker to tick at least once

	if mockProposer.proposalCounter == 0 {
		t.Error("Expected proposerService.CreateProposalBlock to be called, was not.")
	}
	// Check that no block was broadcasted (as proposal failed)
	if len(mockNet.GetBroadcastedBlocks()) > 0 {
		t.Error("No block should have been broadcasted if proposal failed.")
	}
}

// TestConsensusEngine_AddBlock_Failure simulates blockchain.AddBlock failing
// and checks if the state is correctly not updated.
func TestConsensusEngine_AddBlock_Failure(t *testing.T) {
	validatorAddr := []byte("validator_fail_add")
	engine, mockNet, _, _, mockState, mockBlockchain := setupConsensusEngineTest(t, validatorAddr, 0)
	mockBlockchain.addBlockErr = errors.New("simulated add block failure") // Mock blockchain to fail on AddBlock

	if err := engine.Start(); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}
	defer engine.Stop()

	// Simulate an incoming valid block that blockchain.AddBlock will reject
	txs := []core.Transaction{core.Transaction{ID: []byte("tx_add_fail")}}
	incomingBlock := core.NewBlock(1, mockBlockchain.lastBlockHash, txs)
	incomingBlock.Timestamp = time.Now().UnixNano() + 10
	incomingBlock.ProposerAddress = []byte("other_proposer")
	incomingBlock.Sign([]byte("other_proposer"), []byte("dummy_key"))
	incomingBlock.SetHash()

	mockNet.recvChan <- incomingBlock

	time.Sleep(200 * time.Millisecond) // Give time for processing

	// Assertions: Chain height and state height should NOT change (because AddBlock failed)
	if mockBlockchain.ChainHeight() != 0 {
		t.Errorf("Expected blockchain height to remain 0, got %d", mockBlockchain.ChainHeight())
	}
	if mockState.CurrentHeight() != 0 {
		t.Errorf("Expected consensus state height to remain 0, got %d", mockState.CurrentHeight())
	}
	// Ensure the failing block was NOT added to the blockchain's internal map
	_, err := mockBlockchain.GetBlockByHash(incomingBlock.Hash)
	if err == nil {
		t.Errorf("Expected failing block NOT to be found in blockchain, but it was.")
	}
}

// TestConsensusEngine_UpdateHeight_Failure simulates consensusState.UpdateHeight failing
// after a block has been added to the blockchain.
func TestConsensusEngine_UpdateHeight_Failure(t *testing.T) {
	validatorAddr := []byte("validator_fail_update_height")
	engine, mockNet, _, _, mockState, mockBlockchain := setupConsensusEngineTest(t, validatorAddr, 0)

	// Inject a mock for UpdateHeight that simulates failure
	originalUpdateHeight := mockState.UpdateHeight
	updateHeightCalled := make(chan bool, 1)
	mockState.UpdateHeight = func(height int64) error {
		updateHeightCalled <- true
		return errors.New("simulated UpdateHeight error")
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("Engine start failed: %v", err)
	}
	defer engine.Stop()
	defer func() { mockState.UpdateHeight = originalUpdateHeight }() // Restore original mock after test

	// Simulate an incoming block that passes validation and AddBlock, but fails UpdateHeight
	txs := []core.Transaction{core.Transaction{ID: []byte("tx_update_fail")}}
	incomingBlock := core.NewBlock(1, mockBlockchain.lastBlockHash, txs)
	incomingBlock.Timestamp = time.Now().UnixNano() + 10
	incomingBlock.ProposerAddress = []byte("other_proposer_2")
	incomingBlock.Sign([]byte("other_proposer_2"), []byte("dummy_key_2"))
	incomingBlock.SetHash()

	// Crucial: Need to ensure blockchain.AddBlock succeeds so UpdateHeight is called.
	// Temporarily unset addBlockErr if it was set by a previous test.
	mockBlockchain.addBlockErr = nil 

	mockNet.recvChan <- incomingBlock

	time.Sleep(200 * time.Millisecond) // Give time for processing

	// Assertions: Blockchain height should have updated, but ConsensusState height should NOT
	if mockBlockchain.ChainHeight() != 1 {
		t.Errorf("Expected blockchain height to be 1, got %d", mockBlockchain.ChainHeight())
	}
	// This is the key check: ConsensusState height should remain 0 because UpdateHeight failed
	if mockState.CurrentHeight() != 0 { // This confirms the mock's UpdateHeight was called and failed
		t.Errorf("Expected consensus state height to remain 0, got %d", mockState.CurrentHeight())
	}
	// Verify UpdateHeight was actually called
	select {
	case <-updateHeightCalled:
		// Success, the mock was called
	case <-time.After(50 * time.Millisecond):
		t.Error("Expected UpdateHeight to be called, but it wasn't.")
	}
}