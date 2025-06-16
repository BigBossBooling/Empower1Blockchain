package main

import (
	"bytes"
	"empower1.com/core/internal/consensus"
	"empower1.com/core/internal/core"
	"empower1.com/core/internal/crypto" // New import
	"empower1.com/core/internal/mempool" // New import
	"empower1.com/core/internal/p2p"
	"encoding/base64" // For decoding []byte fields from JSON
	"encoding/gob"
	"encoding/hex" // For logging tx IDs
	"encoding/json" // For handling JSON RPC
	"flag"
	"fmt"
	"io/ioutil" // For reading request body
	"log"
	"net/http" // For debug endpoint
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// AppContext holds shared components
type AppContext struct {
	Server            *p2p.Server
	Blockchain        *core.Blockchain
	ConsensusState    *consensus.ConsensusState
	ProposerService   *consensus.ProposerService
	ValidationService *consensus.ValidationService
	Mempool           *mempool.Mempool // Added Mempool
	NodeValidator     *consensus.Validator
	NodePrivateKey    []byte // Placeholder for this node's ECDSA private key bytes
	NodeWalletKey     *crypto.WalletKey // Added for creating transactions
	Config            *AppConfig
	mu                sync.RWMutex
}

// AppConfig holds node configuration
type AppConfig struct {
	ListenPort       string
	InitialPeersRaw  string
	IsValidator      bool
	ValidatorAddress string
	DebugListenAddr  string // For HTTP debug endpoint
}

var appCtx *AppContext

// WalletKey represents a single ECDSA key pair for a user/node wallet.
// Moved to internal/crypto/wallet.go or similar would be better in larger app.
// type WalletKey struct {
// 	PrivateKey *ecdsa.PrivateKey
// 	PublicKey  *ecdsa.PublicKey
// 	Address    string // Hex representation of serialized public key
// }

func main() {
	// Configuration flags
	listenPort := flag.String("listen", ":8080", "Port for P2P connections")
	initialPeersRaw := flag.String("connect", "", "Comma-separated initial peers")
	isValidatorFlag := flag.Bool("validator", false, "Is this node a validator?")
	validatorAddressFlag := flag.String("validator-addr", "node-default-addr", "This node's validator address")
	debugListenAddrFlag := flag.String("debug-listen", ":18080", "Listen address for debug HTTP endpoint")
	flag.Parse()

	appConfig := &AppConfig{
		ListenPort:       *listenPort,
		InitialPeersRaw:  *initialPeersRaw,
		IsValidator:      *isValidatorFlag,
		ValidatorAddress: *validatorAddressFlag,
		DebugListenAddr:  *debugListenAddrFlag,
	}

	fmt.Printf("Hello Empower1 - Node starting...\n")
	log.Printf("Config: Listen=%s, Connect=%s, IsValidator=%t, ValidatorAddr=%s, DebugListen=%s\n",
		appConfig.ListenPort, appConfig.InitialPeersRaw, appConfig.IsValidator, appConfig.ValidatorAddress, appConfig.DebugListenAddr)

	// Initialize core components
	blockchain := core.NewBlockchain()
	consensusState := consensus.NewConsensusState()
	mpool := mempool.NewMempool(1000) // Initialize Mempool

	initialValidators := []*consensus.Validator{
		consensus.NewValidator("validator-1-addr", 100), // These are placeholder addresses
		consensus.NewValidator("validator-2-addr", 100), // In reality, these would be derived from actual pubkeys
		consensus.NewValidator("validator-3-addr", 100),
	}
	consensusState.LoadInitialValidators(initialValidators)
	// Ensure genesis block from blockchain is used to set initial consensus state height
	genesisBlk, err := blockchain.GetBlockByHeight(0)
	if err != nil {
		log.Fatalf("Failed to get genesis block: %v", err)
	}
	consensusState.SetCurrentBlock(genesisBlk)


	validationService := consensus.NewValidationService(consensusState, blockchain)
	var proposerService *consensus.ProposerService
	var nodeValidatorInfo *consensus.Validator
	var nodePrivKeyBytes []byte // Placeholder for raw private key bytes for validator signing (not tx)

	// Generate or load a wallet key for this node (for creating transactions)
	// For simplicity, generating a new one each time. In practice, load from file.
	nodeWalletKey, err := crypto.NewWalletKey()
	if err != nil {
		log.Fatalf("Failed to generate node wallet key: %v", err)
	}
	log.Printf("Node Wallet Address (for sending/receiving txs): %s\n", nodeWalletKey.Address())


	if appConfig.IsValidator {
		found := false
		for _, v := range initialValidators {
			if v.Address == appConfig.ValidatorAddress {
				nodeValidatorInfo = v
				found = true
				break
			}
		}
		if !found {
			log.Printf("WARN: Node configured as validator but address '%s' not in initial validator set. Will not be able to propose.", appConfig.ValidatorAddress)
			appConfig.IsValidator = false
		} else {
			log.Printf("Node is configured as validator: %s (This is a conceptual validator ID, not ECDSA address)\n", nodeValidatorInfo.Address)
			// This private key is for block signing (currently placeholder string based in block.go)
			nodePrivKeyBytes = []byte("privkey-for-" + nodeValidatorInfo.Address)
			// Pass the mempool to the ProposerService
			proposerService = consensus.NewProposerService(nodeValidatorInfo.Address, nodePrivKeyBytes, mpool)
		}
	}

	p2pServer := p2p.NewServer(appConfig.ListenPort)

	appCtx = &AppContext{
		Server:            p2pServer,
		Blockchain:        blockchain,
		ConsensusState:    consensusState,
		ProposerService:   proposerService,
		ValidationService: validationService,
		Mempool:           mpool, // Store mempool in context
		NodeValidator:     nodeValidatorInfo,
		NodePrivateKey:    nodePrivKeyBytes, // This is for block signing
		NodeWalletKey:     nodeWalletKey,    // This is for transaction signing
		Config:            appConfig,
	}

	p2pServer.OnPeerConnected = onPeerConnected
	p2pServer.OnPeerDisconnected = onPeerDisconnected
	p2pServer.OnMessage = onMessageReceived

	if err := p2pServer.Start(); err != nil {
		log.Fatalf("Failed to start P2P server: %v", err)
	}

	connectToInitialPeers(appConfig.InitialPeersRaw, p2pServer, appConfig.ListenPort)

	if appCtx.Config.IsValidator && appCtx.ProposerService != nil {
		go runConsensusLoop()
	}

	// Start HTTP Server (RPC & Debug)
	go startHTTPServer(appConfig.DebugListenAddr)

	waitForSignalAndShutdown()
}

func onPeerConnected(p *p2p.Peer) {
	log.Printf("MAIN: Peer connected: %s. Total peers: %d\n", p.Address(), len(appCtx.Server.KnownPeers()))
}

func onPeerDisconnected(p *p2p.Peer) {
	log.Printf("MAIN: Peer disconnected: %s. Total peers: %d\n", p.Address(), len(appCtx.Server.KnownPeers()))
}

func connectToInitialPeers(initialPeersRaw string, server *p2p.Server, listenPort string) {
	if initialPeersRaw != "" {
		initialPeers := strings.Split(initialPeersRaw, ",")
		for _, peerAddr := range initialPeers {
			trimmedAddr := strings.TrimSpace(peerAddr)
			if trimmedAddr != "" && trimmedAddr != listenPort && trimmedAddr != server.ListenAddress() {
				go func(addr string) {
					log.Printf("MAIN: Attempting to connect to initial peer %s\n", addr)
					_, err := server.Connect(addr)
					if err != nil {
						// log.Printf("MAIN: Failed to connect to initial peer %s: %v", addr, err)
					}
				}(trimmedAddr)
			}
		}
	}
}

func waitForSignalAndShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	running := true
	for running {
		select {
		case <-sigChan:
			log.Println("MAIN: Termination signal received. Shutting down...")
			appCtx.Server.Stop()
			running = false
		case t := <-ticker.C:
			lastBlock, _ := appCtx.Blockchain.GetLastBlock()
			log.Printf("MAIN: Node running [%s]. Height: %d, Peers: %d, Mempool: %d txs, ProposerForNext: %s\n",
				t.Format(time.RFC1123),
				appCtx.Blockchain.ChainHeight(),
				len(appCtx.Server.KnownPeers()),
				appCtx.Mempool.Count(),
				getScheduledProposer(lastBlock.Height+1),
			)
		case <-appCtx.Server.QuitSignal():
			log.Println("MAIN: P2P Server quit unexpectedly. Shutting down application.")
			running = false
		}
	}
	log.Println("MAIN: Application shut down.")
}

func getScheduledProposer(height int64) string {
	proposer, err := appCtx.ConsensusState.GetProposerForHeight(height)
	if err != nil || proposer == nil {
		return "N/A"
	}
	return proposer.Address
}

func runConsensusLoop() {
	proposalTicker := time.NewTicker(10 * time.Second) // Adjusted ticker for proposing
	defer proposalTicker.Stop()
	log.Printf("MAIN: [%s] Starting consensus loop (validator active).\n", appCtx.NodeValidator.Address)

	for {
		select {
		case <-proposalTicker.C:
			tryProposeBlock()
		case <-appCtx.Server.QuitSignal():
			log.Printf("MAIN: [%s] P2P server stopped, exiting consensus loop.\n", appCtx.NodeValidator.Address)
			return
		}
	}
}

func tryProposeBlock() {
	appCtx.mu.RLock()
	if !appCtx.Config.IsValidator || appCtx.ProposerService == nil || appCtx.NodeValidator == nil {
		appCtx.mu.RUnlock()
		return
	}
	lastBlock, err := appCtx.Blockchain.GetLastBlock()
	if err != nil {
		log.Printf("CONSENSUS [%s]: Error getting last block: %v\n", appCtx.NodeValidator.Address, err)
		appCtx.mu.RUnlock()
		return
	}
	appCtx.mu.RUnlock()

	nextHeight := lastBlock.Height + 1
	proposer, err := appCtx.ConsensusState.GetProposerForHeight(nextHeight)
	if err != nil {
		log.Printf("CONSENSUS [%s]: Error getting proposer for height %d: %v\n", appCtx.NodeValidator.Address, nextHeight, err)
		return
	}

	if proposer.Address == appCtx.NodeValidator.Address {
		log.Printf("CONSENSUS [%s]: It's our turn to propose block #%d.\n", appCtx.NodeValidator.Address, nextHeight)
		appCtx.mu.RLock()
		// Pass mempool to CreateProposalBlock (ProposerService already has it)
		newBlock, err := appCtx.ProposerService.CreateProposalBlock(nextHeight, lastBlock.Hash, lastBlock.Timestamp)
		appCtx.mu.RUnlock()

		if err != nil {
			log.Printf("CONSENSUS [%s]: Error creating proposal block #%d: %v\n", appCtx.NodeValidator.Address, nextHeight, err)
			return
		}
		if err := appCtx.ValidationService.ValidateBlock(newBlock); err != nil {
			log.Printf("CONSENSUS [%s]: Proposed block #%d failed self-validation: %v. NOT broadcasting.\n", appCtx.NodeValidator.Address, newBlock.Height, err)
			return
		}
		log.Printf("CONSENSUS [%s]: Successfully created and self-validated block proposal #%d with %d txs. Hash: %x\n",
			appCtx.NodeValidator.Address, newBlock.Height, countTransactionsInBlock(newBlock.Data), newBlock.Hash)

		appCtx.mu.Lock()
		if err := appCtx.Blockchain.AddBlock(newBlock); err != nil {
			log.Printf("CONSENSUS [%s]: Error adding self-proposed block #%d to own chain: %v\n", appCtx.NodeValidator.Address, newBlock.Height, err)
			appCtx.mu.Unlock()
			return
		}
		appCtx.ConsensusState.SetCurrentBlock(newBlock)
		// Remove mined transactions from mempool
		if len(newBlock.Data) > 0 {
			var txsInBlock []*core.Transaction
			if err := gob.NewDecoder(bytes.NewReader(newBlock.Data)).Decode(&txsInBlock); err == nil {
				appCtx.Mempool.RemoveTransactions(txsInBlock)
				log.Printf("CONSENSUS [%s]: Removed %d transactions from mempool after mining block #%d.\n", appCtx.NodeValidator.Address, len(txsInBlock), newBlock.Height) // Corrected Logf: Added appCtx.NodeValidator.Address
			} else {
				log.Printf("CONSENSUS [%s]: Error decoding transactions from self-mined block #%d for mempool removal: %v\n", appCtx.NodeValidator.Address, newBlock.Height, err) // Corrected Logf: Added appCtx.NodeValidator.Address
			}
		}
		log.Printf("CONSENSUS [%s]: Added self-proposed block #%d to own chain. New chain height: %d.\n", appCtx.NodeValidator.Address, newBlock.Height, appCtx.Blockchain.ChainHeight())
		appCtx.mu.Unlock()
		broadcastNewBlock(newBlock)
	}
}

func countTransactionsInBlock(blockData []byte) int {
	if len(blockData) == 0 {
		return 0
	}
	var txs []*core.Transaction
	if err := gob.NewDecoder(bytes.NewReader(blockData)).Decode(&txs); err != nil {
		return 0 // Or log error
	}
	return len(txs)
}

func broadcastNewBlock(block *core.Block) {
	blockBytes, err := gobEncodeBlock(block)
	if err != nil {
		log.Printf("P2P_MSG: Error serializing block for broadcast: %v\n", err)
		return
	}
	proposalPayload := p2p.NewBlockProposalPayload{BlockData: blockBytes}
	payloadBytes, err := p2p.ToBytes(proposalPayload)
	if err != nil {
		log.Printf("P2P_MSG: Error serializing NewBlockProposalPayload: %v\n", err)
		return
	}
	msg := p2p.Message{Type: p2p.MsgNewBlockProposal, Payload: payloadBytes}
	log.Printf("P2P_MSG: Broadcasting MsgNewBlockProposal for block #%d (%x) to peers.\n", block.Height, block.Hash)
	appCtx.Server.BroadcastMessage(&msg, nil)
}

func gobEncodeBlock(block *core.Block) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(block); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gobDecodeBlock(data []byte) (*core.Block, error) {
	var block core.Block
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&block); err != nil {
		return nil, err
	}
	return &block, nil
}

func onMessageReceived(peer *p2p.Peer, msg *p2p.Message) {
	switch msg.Type {
	case p2p.MsgPeerList:
		handlePeerListMsg(peer, msg.Payload)
	case p2p.MsgRequestPeerList:
		handleRequestPeerListMsg(peer)
	case p2p.MsgNewBlockProposal:
		handleNewBlockProposalMsg(peer, msg.Payload)
	case p2p.MsgNewTransaction: // Handle new transaction message
		handleNewTransactionMsg(peer, msg.Payload)
	case p2p.MsgBlockVote:
		log.Printf("P2P_MSG: Received MsgBlockVote from %s (currently placeholder, ignored).\n", peer.Address())
	default:
		log.Printf("P2P_MSG: Received unhandled message type %s from %s\n", msg.Type.String(), peer.Address())
	}
}

func handlePeerListMsg(p *p2p.Peer, payloadBytes []byte) {
	payload, err := p2p.DeserializePeerListPayload(payloadBytes)
	if err != nil {
		log.Printf("P2P_MSG: Error deserializing PEER_LIST payload from %s: %v", p.Address(), err)
		return
	}
	// log.Printf("P2P_MSG: Received PEER_LIST from %s with %d peers.", p.Address(), len(payload.Peers))
	appCtx.mu.RLock()
	myListenAddr := appCtx.Config.ListenPort
	knownPeers := appCtx.Server.KnownPeers()
	appCtx.mu.RUnlock()

	for _, peerAddr := range payload.Peers {
		if peerAddr == myListenAddr || peerAddr == appCtx.Server.ListenAddress() {
			continue
		}
		isAlreadyConnected := false
		for _, cp := range knownPeers {
			if cp == peerAddr {
				isAlreadyConnected = true
				break
			}
		}
		if !isAlreadyConnected {
			// log.Printf("P2P_MSG: Discovered new potential peer %s from %s. Attempting to connect.", peerAddr, p.Address())
			go func(addr string) {
				appCtx.mu.RLock()
				alreadyConnectedCheck := false
				for _, cpCurrent := range appCtx.Server.KnownPeers() {
					if cpCurrent == addr {
						alreadyConnectedCheck = true
						break
					}
				}
				appCtx.mu.RUnlock()
				if alreadyConnectedCheck {
					return
				}
				_, err := appCtx.Server.Connect(addr)
				if err != nil {
					// log.Printf("P2P_MSG: Failed to connect to discovered peer %s: %v", addr, err)
				}
			}(peerAddr)
		}
	}
}

func handleRequestPeerListMsg(p *p2p.Peer) {
	// log.Printf("P2P_MSG: Received PEER_LIST_REQUEST from %s. Sending my peer list.", p.Address())
	appCtx.mu.RLock()
	currentPeers := appCtx.Server.KnownPeers()
	appCtx.mu.RUnlock()
	peersToSend := make([]string, 0, len(currentPeers))
	for _, cpAddr := range currentPeers {
		if cpAddr != p.Address() {
			peersToSend = append(peersToSend, cpAddr)
		}
	}
	if err := appCtx.Server.SendPeerList(p, peersToSend); err != nil {
		log.Printf("P2P_MSG: Error sending peer list to %s: %v", p.Address(), err)
	}
}

func handleNewBlockProposalMsg(p *p2p.Peer, payloadBytes []byte) {
	proposalPayload, err := p2p.DeserializeNewBlockProposalPayload(payloadBytes)
	if err != nil {
		log.Printf("P2P_MSG: Error deserializing NewBlockProposalPayload from %s: %v", p.Address(), err)
		return
	}
	proposedBlock, err := gobDecodeBlock(proposalPayload.BlockData)
	if err != nil {
		log.Printf("P2P_MSG: Error deserializing block from NewBlockProposalPayload from %s: %v", p.Address(), err)
		return
	}
	log.Printf("P2P_MSG: Received MsgNewBlockProposal for block #%d (%x) with %d txs from %s.\n",
		proposedBlock.Height, proposedBlock.Hash, countTransactionsInBlock(proposedBlock.Data), p.Address())

	appCtx.mu.Lock()
	defer appCtx.mu.Unlock()

	if _, err := appCtx.Blockchain.GetBlockByHash(proposedBlock.Hash); err == nil {
		return
	}
	currentChainHeight := appCtx.Blockchain.ChainHeight()
	if proposedBlock.Height <= currentChainHeight {
		return
	}
	if proposedBlock.Height > currentChainHeight+10 {
		log.Printf("P2P_MSG: Received block #%d is too far ahead of current chain height %d. Ignoring for now.\n", proposedBlock.Height, currentChainHeight)
		return
	}

	err = appCtx.ValidationService.ValidateBlock(proposedBlock)
	if err != nil {
		log.Printf("P2P_MSG: Validation failed for block #%d (%x) from %s: %v\n", proposedBlock.Height, proposedBlock.Hash, p.Address(), err)
		return
	}
	log.Printf("P2P_MSG: Successfully validated block #%d (%x) from %s.\n", proposedBlock.Height, proposedBlock.Hash, p.Address())

	err = appCtx.Blockchain.AddBlock(proposedBlock)
	if err != nil {
		log.Printf("P2P_MSG: Error adding validated block #%d (%x) to chain: %v\n", proposedBlock.Height, proposedBlock.Hash, err)
		return
	}
	appCtx.ConsensusState.SetCurrentBlock(proposedBlock)
	log.Printf("P2P_MSG: Added block #%d (%x) to chain. New chain height: %d.\n", proposedBlock.Height, proposedBlock.Hash, appCtx.Blockchain.ChainHeight())

	// Remove transactions from mempool that are now in the accepted block
	if len(proposedBlock.Data) > 0 {
		var txsInBlock []*core.Transaction
		if err := gob.NewDecoder(bytes.NewReader(proposedBlock.Data)).Decode(&txsInBlock); err == nil {
			appCtx.Mempool.RemoveTransactions(txsInBlock)
			log.Printf("P2P_MSG: Removed %d transactions from mempool after processing block #%d from %s.\n", len(txsInBlock), proposedBlock.Height, p.Address())
		} else {
			log.Printf("P2P_MSG: Error decoding transactions from received block #%d for mempool removal: %v\n", proposedBlock.Height, err)
		}
	}

	// Unlock is handled by defer, now broadcast
	appCtx.mu.RUnlock() // Release lock before broadcast to avoid deadlock
	appCtx.mu.RLock()   // Re-acquire read lock for broadcast

	finalBlockBytes, err := gobEncodeBlock(proposedBlock)
	if err != nil {
		log.Printf("P2P_MSG: Error serializing block for gossip: %v\n", err)
		return
	}
	finalProposalPayload := p2p.NewBlockProposalPayload{BlockData: finalBlockBytes}
	finalPayloadBytes, err := p2p.ToBytes(finalProposalPayload)
	if err != nil {
		log.Printf("P2P_MSG: Error serializing NewBlockProposalPayload for gossip: %v", err)
		return
	}
	gossipMsg := p2p.Message{Type: p2p.MsgNewBlockProposal, Payload: finalPayloadBytes}
	appCtx.Server.BroadcastMessage(&gossipMsg, p)
}

func handleNewTransactionMsg(p *p2p.Peer, payloadBytes []byte) {
	txPayload, err := p2p.DeserializeNewTransactionPayload(payloadBytes)
	if err != nil {
		log.Printf("P2P_MSG: Error deserializing NewTransactionPayload from %s: %v", p.Address(), err)
		return
	}
	tx, err := core.DeserializeTransaction(txPayload.TransactionData)
	if err != nil {
		log.Printf("P2P_MSG: Error deserializing transaction from payload from %s: %v", p.Address(), err)
		return
	}

	log.Printf("P2P_MSG: Received MsgNewTransaction %x from %s (Amount: %d, From: %s...)\n",
		tx.ID, p.Address(), tx.Amount, hex.EncodeToString(tx.From[:min(20, len(tx.From))]))

	// Add to mempool (AddTransaction includes signature validation)
	if err := appCtx.Mempool.AddTransaction(tx); err != nil {
		// log.Printf("P2P_MSG: Failed to add transaction %x from %s to mempool: %v\n", tx.ID, p.Address(), err)
		// Don't gossip invalid or duplicate transactions
		return
	}
	log.Printf("P2P_MSG: Added transaction %x from %s to mempool. Mempool size: %d\n", tx.ID, p.Address(), appCtx.Mempool.Count())

	// Gossip the valid new transaction to other peers
	// (excluding the peer who sent it to us)
	gossipTxMsg := p2p.Message{Type: p2p.MsgNewTransaction, Payload: payloadBytes} // Reuse original payload
	appCtx.Server.BroadcastMessage(&gossipTxMsg, p)
	// log.Printf("P2P_MSG: Gossiped transaction %x from %s to other peers.\n", tx.ID, p.Address())
}

// --- HTTP Server (Debug and RPC) ---
func startHTTPServer(listenAddr string) {
	// Debug/Info endpoints
	http.HandleFunc("/create-test-tx", handleCreateTestTx) // Stays for easy testing
	http.HandleFunc("/mempool", handleViewMempool)
	http.HandleFunc("/info", handleNodeInfo) // Basic node info

	// Transaction submission RPC endpoint
	http.HandleFunc("/tx/submit", handleSubmitTransaction) // New RPC endpoint

	log.Printf("MAIN: Starting HTTP server (RPC & Debug) on %s\n", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Printf("MAIN: HTTP server error: %v\n", err)
	}
}

// Go representation of the JSON payload from Python CLI for a transaction
type SubmitTxPayload struct {
	ID        string `json:"ID"`    // Base64 encoded
	Timestamp int64  `json:"Timestamp"`
	From      string `json:"From"` // Base64 encoded
	To        string `json:"To"`   // Base64 encoded
	Amount    uint64 `json:"Amount"`
	Fee       uint64 `json:"Fee"`
	Signature string `json:"Signature"` // Base64 encoded
	PublicKey string `json:"PublicKey"` // Base64 encoded
}

func handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var payload SubmitTxPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	// Convert base64 encoded fields to []byte for core.Transaction
	idBytes, err := base64.StdEncoding.DecodeString(payload.ID)
	if err != nil { http.Error(w, "Invalid ID encoding", http.StatusBadRequest); return }
	fromBytes, err := base64.StdEncoding.DecodeString(payload.From)
	if err != nil { http.Error(w, "Invalid From encoding", http.StatusBadRequest); return }
	toBytes, err := base64.StdEncoding.DecodeString(payload.To)
	if err != nil { http.Error(w, "Invalid To encoding", http.StatusBadRequest); return }
	sigBytes, err := base64.StdEncoding.DecodeString(payload.Signature)
	if err != nil { http.Error(w, "Invalid Signature encoding", http.StatusBadRequest); return }
	pubKeyBytes, err := base64.StdEncoding.DecodeString(payload.PublicKey)
	if err != nil { http.Error(w, "Invalid PublicKey encoding", http.StatusBadRequest); return }

	tx := &core.Transaction{
		ID:        idBytes,
		Timestamp: payload.Timestamp,
		From:      fromBytes,
		To:        toBytes,
		Amount:    payload.Amount,
		Fee:       payload.Fee,
		Signature: sigBytes,
		PublicKey: pubKeyBytes,
	}

	// Validate the transaction (this should include signature check, etc.)
	// The Hash() method in Go core.Transaction now uses JSON canonical form.
	// VerifySignature() uses ecdsa.VerifyASN1, compatible with Python's DER.
	isValid, validationErr := tx.VerifySignature() // Assumes tx.Hash() is called internally by VerifySignature
	if validationErr != nil {
		log.Printf("RPC: /tx/submit - Error verifying transaction signature %s: %v\n", hex.EncodeToString(tx.ID), validationErr)
		http.Error(w, fmt.Sprintf("Transaction signature verification error: %v", validationErr), http.StatusBadRequest)
		return
	}
	if !isValid {
		log.Printf("RPC: /tx/submit - Invalid transaction signature %s\n", hex.EncodeToString(tx.ID))
		http.Error(w, "Invalid transaction signature", http.StatusBadRequest)
		return
	}

	// Recalculate hash to ensure ID matches content (important check!)
	// This ensures the ID field wasn't tampered with relative to the content that was signed.
	recalculatedHash, err := tx.Hash()
	if err != nil {
		log.Printf("RPC: /tx/submit - Error recalculating hash for received tx %s: %v\n", hex.EncodeToString(tx.ID), err)
		http.Error(w, "Error recalculating transaction hash", http.StatusInternalServerError)
		return
	}
	if !bytes.Equal(tx.ID, recalculatedHash) {
		log.Printf("RPC: /tx/submit - Transaction ID %s does not match recalculated content hash %s\n", hex.EncodeToString(tx.ID), hex.EncodeToString(recalculatedHash))
		http.Error(w, "Transaction ID does not match content hash", http.StatusBadRequest)
		return
	}


	log.Printf("RPC: /tx/submit - Received valid transaction %s. Amount: %d\n", hex.EncodeToString(tx.ID), tx.Amount)

	// Add to mempool
	if err := appCtx.Mempool.AddTransaction(tx); err != nil {
		log.Printf("RPC: /tx/submit - Failed to add transaction %s to mempool: %v\n", hex.EncodeToString(tx.ID), err)
		// Distinguish between "already in mempool" vs other errors
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "is full") {
			http.Error(w, err.Error(), http.StatusConflict) // 409 Conflict
		} else {
			http.Error(w, fmt.Sprintf("Failed to add to mempool: %v", err), http.StatusInternalServerError)
		}
		return
	}
	log.Printf("RPC: /tx/submit - Added transaction %s to mempool. Mempool size: %d\n", hex.EncodeToString(tx.ID), appCtx.Mempool.Count())

	// Broadcast to peers (P2P message expects serialized transaction data)
	txDataBytes, err := tx.Serialize() // Gob encodes the transaction
	if err != nil {
		log.Printf("RPC: /tx/submit - Failed to serialize transaction %s for P2P broadcast: %v\n", hex.EncodeToString(tx.ID), err)
		// Transaction is in mempool, but broadcast failed. This is an internal issue.
		// Respond positively to client as tx is accepted locally.
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"message": "Transaction accepted locally, P2P broadcast issue", "tx_id": hex.EncodeToString(tx.ID)})
		return
	}

	txPayloadForP2P := p2p.NewTransactionPayload{TransactionData: txDataBytes}
	payloadBytesForP2P, err := p2p.ToBytes(txPayloadForP2P) // Gob encodes the payload struct
	if err != nil {
		log.Printf("RPC: /tx/submit - Failed to serialize NewTransactionPayload for P2P broadcast: %v\n", err)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"message": "Transaction accepted locally, P2P payload serialization issue", "tx_id": hex.EncodeToString(tx.ID)})
		return
	}

	broadcastMsg := p2p.Message{Type: p2p.MsgNewTransaction, Payload: payloadBytesForP2P}
	appCtx.Server.BroadcastMessage(&broadcastMsg, nil) // Broadcast to all peers
	log.Printf("RPC: /tx/submit - Broadcasted transaction %s to P2P network.\n", hex.EncodeToString(tx.ID))

	w.WriteHeader(http.StatusAccepted) // 202 Accepted
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaction accepted and broadcasted", "tx_id": hex.EncodeToString(tx.ID)})
}


func handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	appCtx.mu.RLock()
	defer appCtx.mu.RUnlock()

	lastBlock, _ := appCtx.Blockchain.GetLastBlock()
	info := map[string]interface{}{
		"node_validator_address": appCtx.Config.ValidatorAddress,
		"node_wallet_address":    appCtx.NodeWalletKey.Address(),
		"is_validator":           appCtx.Config.IsValidator,
		"current_height":         appCtx.Blockchain.ChainHeight(),
		"last_block_hash":        hex.EncodeToString(lastBlock.Hash),
		"mempool_size":           appCtx.Mempool.Count(),
		"connected_peers":        len(appCtx.Server.KnownPeers()),
		"known_peer_list":        appCtx.Server.KnownPeers(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}


func handleCreateTestTx(w http.ResponseWriter, r *http.Request) {
	if appCtx.NodeWalletKey == nil {
		http.Error(w, "Node wallet not initialized", http.StatusInternalServerError)
		return
	}

	// Create a dummy recipient wallet key for the test transaction
	recipientWallet, err := crypto.NewWalletKey()
	if err != nil {
		http.Error(w, "Failed to create recipient wallet for test tx", http.StatusInternalServerError)
		return
	}

	// Create a new transaction
	amount := uint64(10) // Example amount
	fee := uint64(1)    // Example fee
	tx, err := core.NewTransaction(appCtx.NodeWalletKey.PublicKey(), recipientWallet.PublicKey(), amount, fee)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create new transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// Sign the transaction
	if err := tx.Sign(appCtx.NodeWalletKey.PrivateKey()); err != nil {
		http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("DEBUG_HTTP: /create-test-tx - Created and signed test transaction ID: %x\n", tx.ID)

	// Add to local mempool
	if err := appCtx.Mempool.AddTransaction(tx); err != nil {
		log.Printf("DEBUG_HTTP: /create-test-tx - Failed to add test transaction %x to local mempool: %v\n", tx.ID, err)
		// Still try to broadcast it
	} else {
		log.Printf("DEBUG_HTTP: /create-test-tx - Added test transaction %x to local mempool. Mempool size: %d\n", tx.ID, appCtx.Mempool.Count())
	}


	// Broadcast the transaction
	txDataBytes, err := tx.Serialize() // This is GOB encoded transaction
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize transaction for broadcast: %v", err), http.StatusInternalServerError)
		return
	}
	txPayload := p2p.NewTransactionPayload{TransactionData: txDataBytes}
	payloadBytes, err := p2p.ToBytes(txPayload) // This GOB encodes the NewTransactionPayload struct
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize NewTransactionPayload: %v", err), http.StatusInternalServerError)
		return
	}

	broadcastMsg := p2p.Message{Type: p2p.MsgNewTransaction, Payload: payloadBytes}
	appCtx.Server.BroadcastMessage(&broadcastMsg, nil) // Broadcast to all peers

	fmt.Fprintf(w, "Test transaction created and broadcasted. ID: %x\nFrom: %s\nTo: %s\nAmount: %d",
		tx.ID, appCtx.NodeWalletKey.Address(), recipientWallet.Address(), tx.Amount)
}

func handleViewMempool(w http.ResponseWriter, r *http.Request) {
	appCtx.mu.RLock()
	pendingTxs := appCtx.Mempool.GetPendingTransactions(appCtx.Mempool.Count()) // Get all
	appCtx.mu.RUnlock()

	fmt.Fprintf(w, "Mempool (count: %d):\n", len(pendingTxs))
	for i, tx := range pendingTxs {
		fmt.Fprintf(w, "%d. ID: %x, From: %s..., To: %s..., Amount: %d, Fee: %d\n",
			i+1, tx.ID, hex.EncodeToString(tx.From[:min(10, len(tx.From))]), hex.EncodeToString(tx.To[:min(10, len(tx.To))]), tx.Amount, tx.Fee)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


func init() {
	gob.Register(&core.Block{})
	gob.Register(&core.Transaction{}) // Important for encoding slices of transactions in block data
	gob.Register(p2p.HelloPayload{})
	gob.Register(p2p.PeerListPayload{})
	gob.Register(p2p.NewBlockProposalPayload{})
	gob.Register(p2p.BlockVotePayload{})
	gob.Register(p2p.NewTransactionPayload{})
	// Need to register the concrete type for ecdsa.PublicKey if it were part of a gob-encoded struct directly
	// gob.Register(&ecdsa.PublicKey{}) // Not strictly needed if we always serialize to bytes first
}
