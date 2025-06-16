package main

import (
	"bytes"
	"empower1.com/core/internal/consensus"
	"empower1.com/core/internal/core"
	"empower1.com/core/internal/crypto"
	"empower1.com/core/internal/mempool"
	"empower1.com/core/internal/p2p"
	"empower1.com/core/internal/state" // Added import
	"empower1.com/core/internal/vm"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
	ValidationService *consensus.ValidationService // Single instance
	Mempool           *mempool.Mempool
	VMService         *vm.VMService
	NodeValidator     *consensus.Validator
	NodePrivateKey    []byte
	NodeWalletKey     *crypto.WalletKey
	Config            *AppConfig
	mu                sync.RWMutex
}

// AppConfig holds node configuration
type AppConfig struct {
	ListenPort       string
	InitialPeersRaw  string
	IsValidator      bool
	ValidatorAddress string
	DebugListenAddr  string
}

var appCtx *AppContext

func main() {
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

	blockchain := core.NewBlockchain()
	consensusState := consensus.NewConsensusState()
	mpool := mempool.NewMempool(1000)
	vmService := vm.NewVMService()

	initialValidators := []*consensus.Validator{
		consensus.NewValidator("validator-1-addr", 100),
		consensus.NewValidator("validator-2-addr", 100),
		consensus.NewValidator("validator-3-addr", 100),
	}
	consensusState.LoadInitialValidators(initialValidators)
	genesisBlk, err := blockchain.GetBlockByHeight(0)
	if err != nil {
		log.Fatalf("Failed to get genesis block: %v", err)
	}
	consensusState.SetCurrentBlock(genesisBlk)

	validationService := consensus.NewValidationService(consensusState, blockchain)
	var proposerService *consensus.ProposerService
	var nodeValidatorInfo *consensus.Validator
	var nodePrivKeyBytes []byte

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
			log.Printf("Node is configured as validator: %s\n", nodeValidatorInfo.Address)
			nodePrivKeyBytes = []byte("privkey-for-" + nodeValidatorInfo.Address)
			proposerService = consensus.NewProposerService(nodeValidatorInfo.Address, nodePrivKeyBytes, mpool)
		}
	}

	p2pServer := p2p.NewServer(appConfig.ListenPort)

	appCtx = &AppContext{
		Server:            p2pServer,
		Blockchain:        blockchain,
		ConsensusState:    consensusState,
		ProposerService:   proposerService,
		ValidationService: validationService, // Corrected: single instance
		Mempool:           mpool,
		VMService:         vmService,
		NodeValidator:     nodeValidatorInfo,
		NodePrivateKey:    nodePrivKeyBytes,
		NodeWalletKey:     nodeWalletKey,
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

	go startHTTPServer(appConfig.DebugListenAddr) // Corrected call

	waitForSignalAndShutdown()
}

// ... (onPeerConnected, onPeerDisconnected, connectToInitialPeers, waitForSignalAndShutdown, getScheduledProposer, runConsensusLoop, tryProposeBlock, countTransactionsInBlock, broadcastNewBlock, gobEncodeBlock, gobDecodeBlock, onMessageReceived, handlePeerListMsg, handleRequestPeerListMsg, handleNewBlockProposalMsg, handleNewTransactionMsg - these functions remain unchanged from previous correct version) ...
// For brevity, I'm not reproducing all unchanged functions here. Assume they are present and correct.
// The following functions are being updated or added:

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
	proposalTicker := time.NewTicker(10 * time.Second)
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
		newBlock, errProposal := appCtx.ProposerService.CreateProposalBlock(nextHeight, lastBlock.Hash, lastBlock.Timestamp)
		appCtx.mu.RUnlock()

		if errProposal != nil {
			log.Printf("CONSENSUS [%s]: Error creating proposal block #%d: %v\n", appCtx.NodeValidator.Address, nextHeight, errProposal)
			return
		}
		if errVal := appCtx.ValidationService.ValidateBlock(newBlock); errVal != nil {
			log.Printf("CONSENSUS [%s]: Proposed block #%d failed self-validation: %v. NOT broadcasting.\n", appCtx.NodeValidator.Address, newBlock.Height, errVal)
			return
		}
		log.Printf("CONSENSUS [%s]: Successfully created and self-validated block proposal #%d with %d txs. Hash: %x\n",
			appCtx.NodeValidator.Address, newBlock.Height, countTransactionsInBlock(newBlock.Data), newBlock.Hash)

		appCtx.mu.Lock()
		if errAdd := appCtx.Blockchain.AddBlock(newBlock); errAdd != nil {
			log.Printf("CONSENSUS [%s]: Error adding self-proposed block #%d to own chain: %v\n", appCtx.NodeValidator.Address, newBlock.Height, errAdd)
			appCtx.mu.Unlock()
			return
		}
		appCtx.ConsensusState.SetCurrentBlock(newBlock)
		if len(newBlock.Data) > 0 {
			var txsInBlock []*core.Transaction
			if errDec := gob.NewDecoder(bytes.NewReader(newBlock.Data)).Decode(&txsInBlock); errDec == nil {
				appCtx.Mempool.RemoveTransactions(txsInBlock)
				log.Printf("CONSENSUS [%s]: Removed %d transactions from mempool after mining block #%d.\n", appCtx.NodeValidator.Address, len(txsInBlock), newBlock.Height)
			} else {
				log.Printf("CONSENSUS [%s]: Error decoding transactions from self-mined block #%d for mempool removal: %v\n", appCtx.NodeValidator.Address, newBlock.Height, errDec)
			}
		}
		log.Printf("CONSENSUS [%s]: Added self-proposed block #%d to own chain. New chain height: %d.\n", appCtx.NodeValidator.Address, newBlock.Height, appCtx.Blockchain.ChainHeight())
		appCtx.mu.Unlock()
		broadcastNewBlock(newBlock)
	}
}

func countTransactionsInBlock(blockData []byte) int {
	if len(blockData) == 0 { return 0 }
	var txs []*core.Transaction
	if err := gob.NewDecoder(bytes.NewReader(blockData)).Decode(&txs); err != nil { return 0 }
	return len(txs)
}

func broadcastNewBlock(block *core.Block) {
	blockBytes, err := gobEncodeBlock(block)
	if err != nil { log.Printf("P2P_MSG: Error serializing block for broadcast: %v\n", err); return }
	proposalPayload := p2p.NewBlockProposalPayload{BlockData: blockBytes}
	payloadBytes, err := p2p.ToBytes(proposalPayload)
	if err != nil { log.Printf("P2P_MSG: Error serializing NewBlockProposalPayload: %v\n", err); return }
	msg := p2p.Message{Type: p2p.MsgNewBlockProposal, Payload: payloadBytes}
	log.Printf("P2P_MSG: Broadcasting MsgNewBlockProposal for block #%d (%x) to peers.\n", block.Height, block.Hash)
	appCtx.Server.BroadcastMessage(&msg, nil)
}

func gobEncodeBlock(block *core.Block) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(block); err != nil { return nil, err }
	return buf.Bytes(), nil
}

func gobDecodeBlock(data []byte) (*core.Block, error) {
	var block core.Block
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&block); err != nil { return nil, err }
	return &block, nil
}

func onMessageReceived(peer *p2p.Peer, msg *p2p.Message) {
	switch msg.Type {
	case p2p.MsgPeerList: handlePeerListMsg(peer, msg.Payload)
	case p2p.MsgRequestPeerList: handleRequestPeerListMsg(peer)
	case p2p.MsgNewBlockProposal: handleNewBlockProposalMsg(peer, msg.Payload)
	case p2p.MsgNewTransaction: handleNewTransactionMsg(peer, msg.Payload)
	case p2p.MsgBlockVote: log.Printf("P2P_MSG: Received MsgBlockVote from %s (placeholder).\n", peer.Address())
	default: log.Printf("P2P_MSG: Received unhandled message type %s from %s\n", msg.Type.String(), peer.Address())
	}
}

func handlePeerListMsg(p *p2p.Peer, payloadBytes []byte) {
	payload, err := p2p.DeserializePeerListPayload(payloadBytes)
	if err != nil { log.Printf("P2P_MSG: Error deserializing PEER_LIST: %v", err); return }
	appCtx.mu.RLock()
	myListenAddr := appCtx.Config.ListenPort
	knownPeers := appCtx.Server.KnownPeers()
	appCtx.mu.RUnlock()
	for _, peerAddr := range payload.Peers {
		if peerAddr == myListenAddr || peerAddr == appCtx.Server.ListenAddress() { continue }
		isAlreadyConnected := false
		for _, cp := range knownPeers { if cp == peerAddr { isAlreadyConnected = true; break } }
		if !isAlreadyConnected {
			go func(addr string) {
				appCtx.mu.RLock()
				alreadyConnectedCheck := false
				for _, cpCurrent := range appCtx.Server.KnownPeers() { if cpCurrent == addr { alreadyConnectedCheck = true; break } }
				appCtx.mu.RUnlock()
				if alreadyConnectedCheck { return }
				_, _ = appCtx.Server.Connect(addr)
			}(peerAddr)
		}
	}
}

func handleRequestPeerListMsg(p *p2p.Peer) {
	appCtx.mu.RLock()
	currentPeers := appCtx.Server.KnownPeers()
	appCtx.mu.RUnlock()
	peersToSend := make([]string, 0, len(currentPeers))
	for _, cpAddr := range currentPeers { if cpAddr != p.Address() { peersToSend = append(peersToSend, cpAddr) } }
	if err := appCtx.Server.SendPeerList(p, peersToSend); err != nil {
		log.Printf("P2P_MSG: Error sending peer list to %s: %v", p.Address(), err)
	}
}

func handleNewBlockProposalMsg(p *p2p.Peer, payloadBytes []byte) {
	proposalPayload, err := p2p.DeserializeNewBlockProposalPayload(payloadBytes)
	if err != nil { log.Printf("P2P_MSG: Error deserializing NewBlockProposalPayload: %v", err); return }
	proposedBlock, err := gobDecodeBlock(proposalPayload.BlockData)
	if err != nil { log.Printf("P2P_MSG: Error deserializing block from NewBlockProposalPayload: %v", err); return }
	log.Printf("P2P_MSG: Received MsgNewBlockProposal for block #%d (%x) with %d txs from %s.\n",
		proposedBlock.Height, proposedBlock.Hash, countTransactionsInBlock(proposedBlock.Data), p.Address())

	appCtx.mu.Lock()
	defer appCtx.mu.Unlock()

	if _, err = appCtx.Blockchain.GetBlockByHash(proposedBlock.Hash); err == nil { return }
	currentChainHeight := appCtx.Blockchain.ChainHeight()
	if proposedBlock.Height <= currentChainHeight { return }
	if proposedBlock.Height > currentChainHeight+10 { log.Printf("P2P_MSG: Received block #%d too far ahead. Ignoring.\n", proposedBlock.Height); return }

	err = appCtx.ValidationService.ValidateBlock(proposedBlock)
	if err != nil { log.Printf("P2P_MSG: Validation failed for block #%d: %v\n", proposedBlock.Height, err); return }
	log.Printf("P2P_MSG: Successfully validated block #%d from %s.\n", proposedBlock.Height, p.Address())

	err = appCtx.Blockchain.AddBlock(proposedBlock)
	if err != nil { log.Printf("P2P_MSG: Error adding block #%d to chain: %v\n", proposedBlock.Height, err); return }
	appCtx.ConsensusState.SetCurrentBlock(proposedBlock)
	log.Printf("P2P_MSG: Added block #%d to chain. New height: %d.\n", proposedBlock.Height, appCtx.Blockchain.ChainHeight())

	if len(proposedBlock.Data) > 0 {
		var txsInBlock []*core.Transaction
		if errDec := gob.NewDecoder(bytes.NewReader(proposedBlock.Data)).Decode(&txsInBlock); errDec == nil {
			appCtx.Mempool.RemoveTransactions(txsInBlock)
			log.Printf("P2P_MSG: Removed %d transactions from mempool after block #%d from %s.\n", len(txsInBlock), proposedBlock.Height, p.Address())
		} else {
			log.Printf("P2P_MSG: Error decoding txs from received block #%d for mempool removal: %v\n", proposedBlock.Height, errDec)
		}
	}
	// Broadcast logic needs to be outside the lock to avoid deadlock if SendMessage blocks or tries to acquire appCtx.mu
	go func(blockToBroadcast *core.Block, senderPeer *p2p.Peer) {
		finalBlockBytes, errEnc := gobEncodeBlock(blockToBroadcast)
		if errEnc != nil { log.Printf("P2P_MSG: Error serializing block for gossip: %v\n", errEnc); return }
		finalProposalPayload := p2p.NewBlockProposalPayload{BlockData: finalBlockBytes}
		finalPayloadBytes, errEnc := p2p.ToBytes(finalProposalPayload)
		if errEnc != nil { log.Printf("P2P_MSG: Error serializing NewBlockProposalPayload for gossip: %v\n", errEnc); return }
		gossipMsg := p2p.Message{Type: p2p.MsgNewBlockProposal, Payload: finalPayloadBytes}
		appCtx.Server.BroadcastMessage(&gossipMsg, senderPeer)
	}(proposedBlock, p)
}

func handleNewTransactionMsg(p *p2p.Peer, payloadBytes []byte) {
	txPayload, err := p2p.DeserializeNewTransactionPayload(payloadBytes)
	if err != nil { log.Printf("P2P_MSG: Error deserializing NewTransactionPayload: %v", err); return }
	tx, err := core.DeserializeTransaction(txPayload.TransactionData)
	if err != nil { log.Printf("P2P_MSG: Error deserializing transaction from payload: %v", err); return }
	log.Printf("P2P_MSG: Received MsgNewTransaction %x from %s (Amount: %d)\n", tx.ID, p.Address(), tx.Amount)
	if err := appCtx.Mempool.AddTransaction(tx); err != nil { return }
	log.Printf("P2P_MSG: Added transaction %x to mempool. Size: %d\n", tx.ID, appCtx.Mempool.Count())
	gossipTxMsg := p2p.Message{Type: p2p.MsgNewTransaction, Payload: payloadBytes}
	appCtx.Server.BroadcastMessage(&gossipTxMsg, p)
}

// --- HTTP Server (Debug and RPC) ---
func startHTTPServer(listenAddr string) {
	http.HandleFunc("/create-test-tx", handleCreateTestTx)
	http.HandleFunc("/mempool", handleViewMempool)
	http.HandleFunc("/info", handleNodeInfo)
	http.HandleFunc("/test-wasm", handleTestWasm)
	http.HandleFunc("/debug/deploy-contract", handleDebugDeployContract)
	http.HandleFunc("/debug/call-contract", handleDebugCallContract)
	http.HandleFunc("/tx/submit", handleSubmitTransaction)
	log.Printf("MAIN: Starting HTTP server (RPC & Debug) on %s\n", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Printf("MAIN: HTTP server error: %v\n", err)
	}
}

type SubmitTxPayload struct {
	ID        string `json:"ID"`
	Timestamp int64  `json:"Timestamp"`
	From      string `json:"From"`
	PublicKey string `json:"PublicKey,omitempty"`
	Signature string `json:"Signature,omitempty"`
	TxType    string `json:"TxType"`
	To     string `json:"To,omitempty"`
	Amount uint64 `json:"Amount,omitempty"`
	Fee    uint64 `json:"Fee"`
	ContractCode          string   `json:"ContractCode,omitempty"`
	TargetContractAddress string   `json:"TargetContractAddress,omitempty"`
	FunctionName          string   `json:"FunctionName,omitempty"`
	Arguments             string   `json:"Arguments,omitempty"`
	RequiredSignatures  uint32                    `json:"RequiredSignatures,omitempty"`
	AuthorizedPublicKeys []string                 `json:"AuthorizedPublicKeys,omitempty"`
	Signers             []SubmitTxSignerInfoPayload `json:"Signers,omitempty"`
}
type SubmitTxSignerInfoPayload struct {
	PublicKey string `json:"PublicKey"`
	Signature string `json:"Signature"`
}

func handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	body, errRead := ioutil.ReadAll(r.Body) // Changed err var name
	if errRead != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var payload SubmitTxPayload
	var errUnmarshal error // Declare errUnmarshal for json.Unmarshal
	errUnmarshal = json.Unmarshal(body, &payload)
	if errUnmarshal != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling JSON payload: %v", errUnmarshal), http.StatusBadRequest)
		return
	}

	var decErr error // Declare decErr for decodeBase64 helper
	decodeBase64 := func(field, value string) ([]byte, bool) {
		if value == "" { return nil, true }
		bytesVal, decErrInternal := base64.StdEncoding.DecodeString(value)
		if decErrInternal != nil {
			decErr = fmt.Errorf("invalid Base64 encoding for field '%s': %v", field, decErrInternal)
			http.Error(w, decErr.Error(), http.StatusBadRequest)
			return nil, false
		}
		return bytesVal, true
	}

	idBytes, ok := decodeBase64("ID", payload.ID); if !ok { return }
	fromBytes, ok := decodeBase64("From", payload.From); if !ok { return }
	var pubKeyBytes []byte
	if payload.PublicKey != "" { pubKeyBytes, ok = decodeBase64("PublicKey", payload.PublicKey); if !ok { return } }
	var sigBytes []byte
	if payload.Signature != "" { sigBytes, ok = decodeBase64("Signature", payload.Signature); if !ok { return } }
	var toBytes []byte
	if payload.To != "" { toBytes, ok = decodeBase64("To", payload.To); if !ok { return } }
	var contractCodeBytes []byte
	if payload.ContractCode != "" { contractCodeBytes, ok = decodeBase64("ContractCode", payload.ContractCode); if !ok { return } }
	var targetContractAddrBytes []byte
	if payload.TargetContractAddress != "" { targetContractAddrBytes, ok = decodeBase64("TargetContractAddress", payload.TargetContractAddress); if !ok { return } }
	var argumentsBytes []byte
	if payload.Arguments != "" { argumentsBytes, ok = decodeBase64("Arguments", payload.Arguments); if !ok { return } }

	authorizedPubKeysBytes := make([][]byte, len(payload.AuthorizedPublicKeys))
	for i, pkStr := range payload.AuthorizedPublicKeys {
		var pkBytes []byte
		pkBytes, ok = decodeBase64(fmt.Sprintf("AuthorizedPublicKeys[%d]", i), pkStr); if !ok { return }
		authorizedPubKeysBytes[i] = pkBytes
	}
	signersInfo := make([]core.SignerInfo, len(payload.Signers))
	for i, sPayload := range payload.Signers {
		var sPubKeyBytes, sSigBytes []byte
		sPubKeyBytes, ok = decodeBase64(fmt.Sprintf("Signers[%d].PublicKey", i), sPayload.PublicKey); if !ok { return }
		sSigBytes, ok = decodeBase64(fmt.Sprintf("Signers[%d].Signature", i), sPayload.Signature); if !ok { return }
		signersInfo[i] = core.SignerInfo{PublicKey: sPubKeyBytes, Signature: sSigBytes}
	}
	if decErr != nil { return } // Check if any decodeBase64 call failed

	tx := &core.Transaction{
		ID: idBytes, Timestamp: payload.Timestamp, From: fromBytes, PublicKey: pubKeyBytes, Signature: sigBytes,
		TxType: core.TransactionType(payload.TxType), To: toBytes, Amount: payload.Amount, Fee: payload.Fee,
		ContractCode: contractCodeBytes, TargetContractAddress: targetContractAddrBytes,
		FunctionName: payload.FunctionName, Arguments: argumentsBytes,
		RequiredSignatures: payload.RequiredSignatures, AuthorizedPublicKeys: authorizedPubKeysBytes, Signers: signersInfo,
	}

	isValid, validationErr := tx.VerifySignature()
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

	recalculatedHash, hashErr := tx.Hash() // Changed err var name
	if hashErr != nil {
		log.Printf("RPC: /tx/submit - Error recalculating hash for received tx %s: %v\n", hex.EncodeToString(tx.ID), hashErr)
		http.Error(w, "Error recalculating transaction hash", http.StatusInternalServerError)
		return
	}
	if !bytes.Equal(tx.ID, recalculatedHash) {
		log.Printf("RPC: /tx/submit - Transaction ID %s does not match recalculated content hash %s\n", hex.EncodeToString(tx.ID), hex.EncodeToString(recalculatedHash))
		http.Error(w, "Transaction ID does not match content hash", http.StatusBadRequest)
		return
	}
	log.Printf("RPC: /tx/submit - Received valid transaction %s (Type: %s). Amount: %d\n", hex.EncodeToString(tx.ID), tx.TxType, tx.Amount)

	switch tx.TxType {
	case core.TxContractDeploy:
		contractAddress := state.GenerateNewContractAddress(tx.From)
		deployErr := state.StoreContractCode(contractAddress, tx.ContractCode) // Changed err var name
		if deployErr != nil {
			log.Printf("RPC: /tx/submit - Failed to store contract code for tx %s: %v\n", hex.EncodeToString(tx.ID), deployErr)
			http.Error(w, fmt.Sprintf("Failed to store contract code: %v", deployErr), http.StatusInternalServerError)
			return
		}
		log.Printf("RPC: /tx/submit - Contract code deployed for tx %s at new address %s\n", hex.EncodeToString(tx.ID), hex.EncodeToString(contractAddress))
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Contract deployed successfully (direct processing)", "tx_id": hex.EncodeToString(tx.ID), "contract_address": hex.EncodeToString(contractAddress),
		})
		return
	case core.TxContractCall:
		gasLimit := uint64(1_000_000)
		log.Printf("RPC: /tx/submit - Executing contract call for tx %s to contract %s, function %s\n", hex.EncodeToString(tx.ID), hex.EncodeToString(tx.TargetContractAddress), tx.FunctionName)
		wasmCode, getCodeErr := state.GetContractCode(tx.TargetContractAddress) // Changed err var name
		if getCodeErr != nil {
			log.Printf("RPC: /tx/submit - Failed to get contract code for %s: %v\n", hex.EncodeToString(tx.TargetContractAddress), getCodeErr)
			http.Error(w, fmt.Sprintf("Failed to get contract code: %v", getCodeErr), http.StatusNotFound)
			return
		}
		vmResult, gasConsumed, execErr := appCtx.VMService.ExecuteContract( tx.TargetContractAddress, wasmCode, tx.FunctionName, gasLimit ) // Simplified args for now
		log.Printf("RPC: /tx/submit - Contract call executed. Gas consumed: %d. Result: %v. Error: %v\n", gasConsumed, vmResult, execErr)
		if execErr != nil {
			if execErr == vm.ErrOutOfGas { http.Error(w, fmt.Sprintf("Contract execution failed: out of gas (consumed: %d)", gasConsumed), http.StatusPaymentRequired)
			} else { http.Error(w, fmt.Sprintf("Contract execution failed: %v (gas consumed: %d)", execErr, gasConsumed), http.StatusInternalServerError) }
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Contract call executed (direct processing)", "tx_id": hex.EncodeToString(tx.ID), "return_value": vmResult, "gas_consumed": gasConsumed,
		})
		return
	case core.TxStandard:
		if addErr := appCtx.Mempool.AddTransaction(tx); addErr != nil { // Changed err var name
			log.Printf("RPC: /tx/submit - Failed to add standard transaction %s to mempool: %v\n", hex.EncodeToString(tx.ID), addErr)
			if strings.Contains(addErr.Error(), "already exists") || strings.Contains(addErr.Error(), "is full") { http.Error(w, addErr.Error(), http.StatusConflict)
			} else { http.Error(w, fmt.Sprintf("Failed to add to mempool: %v", addErr), http.StatusInternalServerError) }
			return
		}
		log.Printf("RPC: /tx/submit - Added standard transaction %s to mempool. Mempool size: %d\n", hex.EncodeToString(tx.ID), appCtx.Mempool.Count())
		broadcastTransaction(tx)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"message": "Standard transaction accepted and broadcasted", "tx_id": hex.EncodeToString(tx.ID)})
		return
	default:
		log.Printf("RPC: /tx/submit - Received transaction with unknown type: %s\n", tx.TxType)
		http.Error(w, fmt.Sprintf("Unknown transaction type: %s", tx.TxType), http.StatusBadRequest)
		return
	}
}

func broadcastTransaction(tx *core.Transaction) {
	txDataBytes, serErr := tx.Serialize() // Changed err var name
	if serErr != nil { log.Printf("P2P_BROADCAST: Failed to serialize transaction %s: %v\n", hex.EncodeToString(tx.ID), serErr); return }
	txPayloadForP2P := p2p.NewTransactionPayload{TransactionData: txDataBytes}
	payloadBytesForP2P, serPayloadErr := p2p.ToBytes(txPayloadForP2P) // Changed err var name
	if serPayloadErr != nil { log.Printf("P2P_BROADCAST: Failed to serialize NewTransactionPayload: %v\n", serPayloadErr); return }
	broadcastMsg := p2p.Message{Type: p2p.MsgNewTransaction, Payload: payloadBytesForP2P}
	appCtx.Server.BroadcastMessage(&broadcastMsg, nil)
	log.Printf("P2P_BROADCAST: Broadcasted transaction %s to P2P network.\n", hex.EncodeToString(tx.ID))
}

func handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	appCtx.mu.RLock()
	defer appCtx.mu.RUnlock()
	lastBlock, _ := appCtx.Blockchain.GetLastBlock()
	info := map[string]interface{}{
		"node_validator_address": appCtx.Config.ValidatorAddress, "node_wallet_address":    appCtx.NodeWalletKey.Address(),
		"is_validator":           appCtx.Config.IsValidator, "current_height":         appCtx.Blockchain.ChainHeight(),
		"last_block_hash":        hex.EncodeToString(lastBlock.Hash), "mempool_size":           appCtx.Mempool.Count(),
		"connected_peers":        len(appCtx.Server.KnownPeers()), "known_peer_list":        appCtx.Server.KnownPeers(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func handleTestWasm(w http.ResponseWriter, r *http.Request) {
	wasmFilePath := "contracts_src/simple_contract/out/simple_contract.wasm"
	wasmCode, errRead := ioutil.ReadFile(wasmFilePath) // Changed err var name
	if errRead != nil {
		http.Error(w, fmt.Sprintf("Error reading WASM file '%s': %v", wasmFilePath, errRead), http.StatusInternalServerError)
		log.Printf("DEBUG_HTTP: /test-wasm - Error reading WASM file: %v", errRead); return
	}
	log.Printf("DEBUG_HTTP: /test-wasm - Read %d bytes from WASM file: %s", len(wasmCode), wasmFilePath)
	if appCtx.VMService == nil { http.Error(w, "VMService not initialized", http.StatusInternalServerError); log.Printf("DEBUG_HTTP: /test-wasm - VMService not initialized"); return }
	var resultsOutput string

	resultsOutput += "Calling 'add(5, 7)':\n"
	log.Printf("DEBUG_HTTP: /test-wasm - Calling 'add(5,7)'...")
	result, gasConsumedAdd, errAdd := appCtx.VMService.ExecuteContract(
		[]byte("test_contract_addr_for_add"), wasmCode, "add", uint64(100000), int32(5), int32(7),
	)
	if errAdd != nil {
		resultsOutput += fmt.Sprintf("  Error executing 'add': %v, Gas Consumed: %d\n", errAdd, gasConsumedAdd)
		log.Printf("DEBUG_HTTP: /test-wasm - Error executing 'add': %v, Gas: %d", errAdd, gasConsumedAdd)
	} else {
		resultsOutput += fmt.Sprintf("  Result of 'add(5, 7)': %v (Type: %T), Gas Consumed: %d\n", result, result, gasConsumedAdd)
		log.Printf("DEBUG_HTTP: /test-wasm - Result of 'add(5, 7)': %v, Gas: %d", result, gasConsumedAdd)
	}

	resultsOutput += "\nAttempting to call 'greet(\"World\")' (argument passing is complex, focus on host log):\n"
	log.Printf("DEBUG_HTTP: /test-wasm - Attempting 'greet(\"World\")'...")
	greetResultPtr, gasConsumedGreet, errGreet := appCtx.VMService.ExecuteContract(
		[]byte("test_contract_addr_for_greet"), wasmCode, "greet", uint64(100000),
		// Passing arguments to AS string parameters from Go is non-trivial and not fully handled here.
		// This call will likely not pass "World" correctly to the WASM 'greet' function.
		// We pass a dummy int32(0) as a placeholder for the string reference.
		// The primary test here is the invocation of host_log_message from within greet.
		int32(0),
	)
	if errGreet != nil {
		resultsOutput += fmt.Sprintf("  Error executing 'greet': %v, Gas Consumed: %d\n", errGreet, gasConsumedGreet)
		log.Printf("DEBUG_HTTP: /test-wasm - Error executing 'greet': %v, Gas: %d", errGreet, gasConsumedGreet)
	} else {
		resultsOutput += fmt.Sprintf("  Result of 'greet' (ptr to string): %v (Type: %T), Gas Consumed: %d\n", greetResultPtr, greetResultPtr, gasConsumedGreet)
		log.Printf("DEBUG_HTTP: /test-wasm - Result of 'greet' (ptr): %v, Gas: %d", greetResultPtr, gasConsumedGreet)
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "WASM Execution Test Results:")
	fmt.Fprintln(w, resultsOutput)
	log.Printf("DEBUG_HTTP: /test-wasm - Test execution finished.")
}

// --- Debug Handlers for Contract Deployment and Calls ---
type DeployContractPayload struct {
	WasmFilePath     string `json:"wasm_file_path"`
	DeployerAddress  string `json:"deployer_address"`
}
func handleDebugDeployContract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed); return }
	var payload DeployContractPayload
	if errDec := json.NewDecoder(r.Body).Decode(&payload); errDec != nil { http.Error(w, fmt.Sprintf("Error decoding JSON: %v", errDec), http.StatusBadRequest); return }
	defer r.Body.Close()
	if payload.WasmFilePath == "" || payload.DeployerAddress == "" { http.Error(w, "wasm_file_path and deployer_address required", http.StatusBadRequest); return }

	wasmCode, errRead := ioutil.ReadFile(payload.WasmFilePath)
	if errRead != nil { http.Error(w, fmt.Sprintf("Error reading WASM '%s': %v", payload.WasmFilePath, errRead), http.StatusInternalServerError); return }
	deployerAddrBytes, errHex := hex.DecodeString(payload.DeployerAddress)
	if errHex != nil { http.Error(w, fmt.Sprintf("Invalid deployer_address: %v", errHex), http.StatusBadRequest); return }

	contractAddress := state.GenerateNewContractAddress(deployerAddrBytes)
	errStore := state.StoreContractCode(contractAddress, wasmCode)
	if errStore != nil {
		log.Printf("RPC: /debug/deploy-contract - Failed to store contract code: %v\n", errStore)
		http.Error(w, fmt.Sprintf("Failed to store contract code: %v", errStore), http.StatusInternalServerError)
		return
	}
	log.Printf("RPC: /debug/deploy-contract - Code from %s deployed to new address %s\n", payload.WasmFilePath, hex.EncodeToString(contractAddress))
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Contract deployed (debug)", "contract_address": hex.EncodeToString(contractAddress), "wasm_file_path": payload.WasmFilePath,
	})
}

type CallContractPayload struct {
	ContractAddress string `json:"contract_address"`
	FunctionName    string `json:"function_name"`
	ArgumentsJSON   string `json:"arguments_json"`
	GasLimit        uint64 `json:"gas_limit"`
}
func handleDebugCallContract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed); return }
	var payload CallContractPayload
	if errDec := json.NewDecoder(r.Body).Decode(&payload); errDec != nil { http.Error(w, fmt.Sprintf("Error decoding JSON: %v", errDec), http.StatusBadRequest); return }
	defer r.Body.Close()

	contractAddrBytes, errHex := hex.DecodeString(payload.ContractAddress)
	if errHex != nil { http.Error(w, fmt.Sprintf("Invalid contract_address: %v", errHex), http.StatusBadRequest); return }
	wasmCode, errGetCode := state.GetContractCode(contractAddrBytes)
	if errGetCode != nil { http.Error(w, fmt.Sprintf("Failed to get contract code for %s: %v", payload.ContractAddress, errGetCode), http.StatusNotFound); return }

	var rawArgs []string // For simple string array from JSON
	if errJsonArgs := json.Unmarshal([]byte(payload.ArgumentsJSON), &rawArgs); errJsonArgs != nil {
		http.Error(w, fmt.Sprintf("Error parsing arguments_json: %v. Expected JSON array of strings.", errJsonArgs), http.StatusBadRequest)
		return
	}

	var finalWasmArgs []interface{}
	// This argument conversion is highly simplified and will NOT correctly marshal strings for AS.
	// It's a placeholder to demonstrate the call path. Real marshalling is needed.
	if payload.FunctionName == "init" && len(rawArgs) == 0 {
		finalWasmArgs = []interface{}{}
	} else { // For set, get, log - they take strings, this is where proper marshalling is needed.
		log.Printf("DEBUG_HTTP: /debug/call-contract - WARNING: Calling '%s'. String argument marshalling from Go to AS is complex and not fully implemented in this debug handler. Contract may not receive string args correctly.", payload.FunctionName)
		for _, argStr := range rawArgs { finalWasmArgs = append(finalWasmArgs, argStr) } // Passing Go strings directly
	}

	log.Printf("DEBUG_HTTP: /debug/call-contract - Calling %s on %s with (simplified/potentially mis-marshalled) args: %v, Gas: %d\n",
		payload.FunctionName, payload.ContractAddress, finalWasmArgs, payload.GasLimit)

	vmResult, gasConsumed, execErr := appCtx.VMService.ExecuteContract(
		contractAddrBytes, wasmCode, payload.FunctionName, payload.GasLimit, finalWasmArgs...
	)
	log.Printf("DEBUG_HTTP: /debug/call-contract - Executed %s. Gas: %d, Result: %v, Err: %v\n", payload.FunctionName, gasConsumed, vmResult, execErr)

	if execErr != nil {
		if execErr == vm.ErrOutOfGas { http.Error(w, fmt.Sprintf("Contract execution failed: out of gas (consumed: %d)", gasConsumed), http.StatusPaymentRequired)
		} else { http.Error(w, fmt.Sprintf("Contract execution error: %v (gas consumed: %d)", execErr, gasConsumed), http.StatusInternalServerError) }
		return
	}
	responsePayload := map[string]interface{}{
		"message": "Contract call attempted", "function_name": payload.FunctionName,
		"contract_result": vmResult, "gas_consumed": gasConsumed,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responsePayload)
}


func handleCreateTestTx(w http.ResponseWriter, r *http.Request) {
	if appCtx.NodeWalletKey == nil { http.Error(w, "Node wallet not initialized", http.StatusInternalServerError); return }
	recipientWallet, errWallet := crypto.NewWalletKey()
	if errWallet != nil { http.Error(w, "Failed to create recipient wallet for test tx", http.StatusInternalServerError); return }

	amount := uint64(10); fee := uint64(1)
	// Using NewStandardTransaction now
	tx, errTx := core.NewStandardTransaction(appCtx.NodeWalletKey.PublicKey(), recipientWallet.PublicKey(), amount, fee)
	if errTx != nil { http.Error(w, fmt.Sprintf("Failed to create new transaction: %v", errTx), http.StatusInternalServerError); return }
	if errSign := tx.Sign(appCtx.NodeWalletKey.PrivateKey()); errSign != nil { http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", errSign), http.StatusInternalServerError); return }
	log.Printf("DEBUG_HTTP: /create-test-tx - Created and signed test transaction ID: %x\n", tx.ID)

	if errAdd := appCtx.Mempool.AddTransaction(tx); errAdd != nil {
		log.Printf("DEBUG_HTTP: /create-test-tx - Failed to add test transaction %x to local mempool: %v\n", tx.ID, errAdd)
	} else {
		log.Printf("DEBUG_HTTP: /create-test-tx - Added test transaction %x to local mempool. Mempool size: %d\n", tx.ID, appCtx.Mempool.Count())
	}

	txDataBytes, errSer := tx.Serialize()
	if errSer != nil { http.Error(w, fmt.Sprintf("Failed to serialize transaction for broadcast: %v", errSer), http.StatusInternalServerError); return }
	txPayload := p2p.NewTransactionPayload{TransactionData: txDataBytes}
	payloadBytes, errToBytes := p2p.ToBytes(txPayload)
	if errToBytes != nil { http.Error(w, fmt.Sprintf("Failed to serialize NewTransactionPayload: %v", errToBytes), http.StatusInternalServerError); return }
	broadcastMsg := p2p.Message{Type: p2p.MsgNewTransaction, Payload: payloadBytes}
	appCtx.Server.BroadcastMessage(&broadcastMsg, nil)
	fmt.Fprintf(w, "Test transaction created and broadcasted. ID: %x\nFrom: %s\nTo: %s\nAmount: %d",
		tx.ID, appCtx.NodeWalletKey.Address(), recipientWallet.Address(), tx.Amount)
}

func handleViewMempool(w http.ResponseWriter, r *http.Request) {
	appCtx.mu.RLock()
	pendingTxs := appCtx.Mempool.GetPendingTransactions(appCtx.Mempool.Count())
	appCtx.mu.RUnlock()
	fmt.Fprintf(w, "Mempool (count: %d):\n", len(pendingTxs))
	for i, tx := range pendingTxs {
		fmt.Fprintf(w, "%d. ID: %x, From: %s..., To: %s..., Amount: %d, Fee: %d, Type: %s\n",
			i+1, tx.ID, hex.EncodeToString(tx.From[:min(10, len(tx.From))]), hex.EncodeToString(tx.To[:min(10, len(tx.To))]), tx.Amount, tx.Fee, tx.TxType)
	}
}

func min(a, b int) int { if a < b { return a }; return b }

func init() {
	gob.Register(&core.Block{})
	gob.Register(&core.Transaction{})
	gob.Register(p2p.HelloPayload{})
	gob.Register(p2p.PeerListPayload{})
	gob.Register(p2p.NewBlockProposalPayload{})
	gob.Register(p2p.BlockVotePayload{})
	gob.Register(p2p.NewTransactionPayload{})
}
