package p2p

import (
	"bytes"
	"encoding/hex"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output
	"sync"
	"time"

	"empower1.com/core/core" // Assuming 'core' is the package alias for empower1.com/core/core
)

// --- Custom Errors for NetworkManager ---
var (
	ErrManagerAlreadyRunning = errors.New("network manager is already running")
	ErrManagerNotRunning     = errors.New("network manager is not running")
	ErrManagerInit           = errors.New("network manager initialization error")
	ErrInvalidPeerAddress    = errors.New("invalid peer address for discovery")
	ErrPeerAlreadyKnown      = errors.New("peer already known for discovery")
)

const (
	minDesiredPeers    = 5             // Minimum number of active connections to maintain
	maxDesiredPeers    = 10            // Maximum number of active connections
	peerDiscoveryInterval = 30 * time.Second // How often to attempt peer discovery
	peerLivenessCheckInterval = 15 * time.Second // How often to check peer liveness
	maxPeerDiscoveryAttempts = 3       // How many times to try connecting to a new discovered peer
	reconnectDelay         = 5 * time.Second // Delay before reconnecting to a known peer
)

// NetworkManager orchestrates peer discovery, connection management, and message routing.
// It acts as the central control plane for the P2P network.
type NetworkManager struct {
	server        *Server             // The underlying P2P Server handling connections
	selfNodeID    []byte              // Our node's cryptographic ID
	knownAddresses map[string][]byte  // All known peer addresses -> Peer ID (for discovery)
	activePeers   map[string]*Peer   // Currently active connections, keyed by address

	incomingBlocks  chan *core.Block     // Channel for blocks from network to consensus engine
	incomingTxs     chan *core.Transaction // Channel for transactions from network to mempool

	stopChan      chan struct{}       // Signal to stop manager goroutines
	isRunning     bool                // Flag indicating running status
	wg            sync.WaitGroup      // WaitGroup for manager goroutines
	logger        *log.Logger         // Dedicated logger for the NetworkManager
}

// NewNetworkManager creates a new NetworkManager instance.
// It sets up the core components and channels for inter-service communication.
func NewNetworkManager(server *Server, selfNodeID []byte, blockChanBuffer, txChanBuffer int) (*NetworkManager, error) {
	if server == nil {
		return nil, fmt.Errorf("%w: server cannot be nil", ErrManagerInit)
	}
	if len(selfNodeID) == 0 {
		return nil, fmt.Errorf("%w: self node ID cannot be empty", ErrManagerInit)
	}
	if blockChanBuffer <= 0 || txChanBuffer <= 0 {
		return nil, fmt.Errorf("%w: channel buffer sizes must be positive", ErrManagerInit)
	}

	logger := log.New(os.Stdout, "NET_MANAGER: ", log.Ldate|log.Ltime|log.Lshortfile)

	nm := &NetworkManager{
		server:        server,
		selfNodeID:    selfNodeID,
		knownAddresses: make(map[string][]byte),
		activePeers:   make(map[string]*Peer),
		
		incomingBlocks: make(chan *core.Block, blockChanBuffer),
		incomingTxs:    make(chan *core.Transaction, txChanBuffer),

		stopChan:      make(chan struct{}),
		isRunning:     false,
		wg:            sync.WaitGroup{},
		logger:        logger,
	}

	// Set up callbacks on the underlying Server (critical for synchronization)
	nm.server.OnPeerConnected = nm.handlePeerConnected
	nm.server.OnPeerDisconnected = nm.handlePeerDisconnected
	nm.server.OnMessage = nm.handleIncomingMessage // Route all messages here for processing

	nm.logger.Println("NetworkManager initialized for node ID:", hex.EncodeToString(selfNodeID))
	return nm, nil
}

// Start initiates the NetworkManager's operation.
// It begins peer discovery, connection management, and message routing loops.
func (nm *NetworkManager) Start() error {
	if nm.isRunning {
		return ErrManagerAlreadyRunning
	}

	// Start the underlying P2P server first
	if err := nm.server.Start(); err != nil {
		return fmt.Errorf("failed to start underlying P2P server: %w", err)
	}

	// Start manager goroutines
	nm.wg.Add(1)
	go nm.peerDiscoveryLoop()
	nm.wg.Add(1)
	go nm.connectionMaintenanceLoop()

	nm.isRunning = true
	nm.logger.Println("NetworkManager started.")
	return nil
}

// Stop gracefully shuts down the NetworkManager.
func (nm *NetworkManager) Stop() error {
	if !nm.isRunning {
		return ErrManagerNotRunning
	}

	close(nm.stopChan)     // Signal manager goroutines to stop
	nm.wg.Wait()           // Wait for manager goroutines to finish

	// Stop the underlying P2P server
	nm.server.Stop() 

	// Close outgoing channels (if any) and incoming public channels
	close(nm.incomingBlocks)
	close(nm.incomingTxs)

	nm.isRunning = false
	nm.logger.Println("NetworkManager stopped.")
	return nil
}

// AddKnownAddress adds a new address to the manager's list of known peers.
// This is used for initial bootstrapping (e.g., seed nodes) or discovery.
func (nm *NetworkManager) AddKnownAddress(address string, nodeID []byte) error {
	if address == "" {
		return ErrInvalidPeerAddress
	}
	addrHex := address // Use address string directly for map key

	// Ensure thread-safe access to knownAddresses map
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if _, exists := nm.knownAddresses[addrHex]; exists {
		nm.logger.Debugf("NETWORK_MANAGER: Address %s already known.", addrHex)
		return ErrPeerAlreadyKnown // Or return nil, depending on desired behavior
	}

	nm.knownAddresses[addrHex] = nodeID // Store the address and associated ID
	nm.logger.Printf("NETWORK_MANAGER: Added new known address: %s (ID: %x)", addrHex, nodeID)
	return nil
}

// GetIncomingBlockChannel returns a channel for consuming incoming blocks.
// This interface is used by the ConsensusEngine.
func (nm *NetworkManager) GetIncomingBlockChannel() <-chan *core.Block {
	return nm.incomingBlocks
}

// GetIncomingTransactionChannel returns a channel for consuming incoming transactions.
// This interface is used by the Mempool.
func (nm *NetworkManager) GetIncomingTransactionChannel() <-chan *core.Transaction {
	return nm.incomingTxs
}

// --- Internal Goroutine Loops ---

// peerDiscoveryLoop actively attempts to find and connect to new peers.
func (nm *NetworkManager) peerDiscoveryLoop() {
	defer nm.wg.Done()
	ticker := time.NewTicker(peerDiscoveryInterval)
	defer ticker.Stop()
	nm.logger.Debug("Peer discovery loop started.")

	for {
		select {
		case <-nm.stopChan:
			nm.logger.Debug("Peer discovery loop received stop signal.")
			return
		case <-ticker.C:
			nm.discoverAndConnect()
		}
	}
}

// connectionMaintenanceLoop actively checks the health of active connections and prunes stale ones.
func (nm *NetworkManager) connectionMaintenanceLoop() {
	defer nm.wg.Done()
	ticker := time.NewTicker(peerLivenessCheckInterval)
	defer ticker.Stop()
	nm.logger.Debug("Connection maintenance loop started.")

	for {
		select {
		case <-nm.stopChan:
			nm.logger.Debug("Connection maintenance loop received stop signal.")
			return
		case <-ticker.C:
			nm.pruneStaleConnections()
		}
	}
}

// --- Peer Callbacks from Server ---

// handlePeerConnected is called by the Server when a new peer successfully completes handshake.
func (nm *NetworkManager) handlePeerConnected(p *Peer) {
	nm.mu.Lock()
	nm.activePeers[p.Address()] = p // Add to active peers map
	nm.mu.Unlock()
	nm.logger.Printf("NETWORK_MANAGER: Peer connected: %s (ID: %x). Total active: %d", p.Address(), p.ID(), len(nm.activePeers))
	
	// Add its address to our knownAddresses if not already there
	nm.AddKnownAddress(p.Address(), p.ID()) // This handles internal locking
	
	// Request peer list from newly connected peer for further discovery
	// This is important for initial network graph building
	if err := nm.server.sendRequestPeerList(p); err != nil { // Assuming sendRequestPeerList is public or a callback is available
		nm.logger.Errorf("NETWORK_MANAGER: Failed to request peer list from new peer %s: %v", p.Address(), err)
	}
}

// handlePeerDisconnected is called by the Server when a peer's connection is closed.
func (nm *NetworkManager) handlePeerDisconnected(p *Peer) {
	nm.mu.Lock()
	delete(nm.activePeers, p.Address()) // Remove from active peers map
	nm.mu.Unlock()
	nm.logger.Printf("NETWORK_MANAGER: Peer disconnected: %s (ID: %x). Total active: %d", p.Address(), p.ID(), len(nm.activePeers))
	// Reconnection logic could be triggered here if this was an outbound connection to a desired peer.
}

// handleIncomingMessage is called by the Server when a message is received from any peer.
// This central handler routes messages to appropriate internal channels or processes them.
func (nm *NetworkManager) handleIncomingMessage(p *Peer, msg *Message) {
	nm.logger.Debugf("NETWORK_MANAGER: Received %s from %s (ID: %x)", msg.Type.String(), p.Address(), p.ID())

	// Update peer's last activity, crucial for liveness checks
	p.UpdateLastActivity()

	switch msg.Type {
	case MsgHello:
		// Already handled by Server.handleConnection's readAndProcessHello.
		// If received again from an active peer, maybe ignore or re-handshake (V2+).
		nm.logger.Debugf("NETWORK_MANAGER: Received duplicate HELLO from %s. Ignoring.", p.Address())
	case MsgPeerList:
		var payload PeerListPayload
		if err := DecodePayload(msg.Payload, &payload); err != nil {
			nm.logger.Errorf("NETWORK_MANAGER: Failed to decode PeerListPayload from %s: %v", p.Address(), err)
			return
		}
		nm.logger.Printf("NETWORK_MANAGER: Received PeerList from %s. Peers: %d", p.Address(), len(payload.Peers))
		for _, addr := range payload.Peers {
			nm.AddKnownAddress(addr, nil) // NodeID might not be in PeerList, resolve later or during connect.
		}
	case MsgNewBlockProposal:
		var payload NewBlockProposalPayload
		if err := DecodePayload(msg.Payload, &payload); err != nil {
			nm.logger.Errorf("NETWORK_MANAGER: Failed to decode NewBlockProposalPayload from %s: %v", p.Address(), err)
			return
		}
		block, err := core.DeserializeBlock(payload.BlockData) // Assuming core.DeserializeBlock is available
		if err != nil {
			nm.logger.Errorf("NETWORK_MANAGER: Failed to deserialize block from proposal from %s: %v", p.Address(), err)
			return
		}
		// Route block to ConsensusEngine via channel
		select {
		case nm.incomingBlocks <- block:
			nm.logger.Debugf("NETWORK_MANAGER: Routed block %x to incomingBlocks channel.", block.Hash)
		default:
			nm.logger.Warnf("NETWORK_MANAGER_WARN: Incoming block channel full, dropping block %x from %s.", block.Hash, p.Address())
		}
		// After receiving, broadcast to other peers if relevant (gossip logic)
		nm.gossipBlock(block, p) // Don't send back to sender
	case MsgNewTransaction:
		var payload NewTransactionPayload
		if err := DecodePayload(msg.Payload, &payload); err != nil {
			nm.logger.Errorf("NETWORK_MANAGER: Failed to decode NewTransactionPayload from %s: %v", p.Address(), err)
			return
		}
		tx, err := core.DeserializeTransaction(payload.TransactionData) // Assuming core.DeserializeTransaction is available
		if err != nil {
			nm.logger.Errorf("NETWORK_MANAGER: Failed to deserialize transaction from %s: %v", p.Address(), err)
			return
		}
		// Route transaction to Mempool via channel
		select {
		case nm.incomingTxs <- tx:
			nm.logger.Debugf("NETWORK_MANAGER: Routed transaction %x to incomingTxs channel.", tx.ID)
		default:
			nm.logger.Warnf("NETWORK_MANAGER_WARN: Incoming transaction channel full, dropping tx %x from %s.", tx.ID, p.Address())
		}
		// After receiving, broadcast to other peers if relevant (gossip logic)
		nm.gossipTransaction(tx, p) // Don't send back to sender
	// Add other message types handlers (MsgBlockRequest, MsgBlockResponse, MsgBlockVote, EmPower1 specific msgs)
	case MsgBlockRequest:
		nm.logger.Debug("NETWORK_MANAGER: Received Block Request. (Conceptual: Respond to request)")
		// In a real system, you would look up the requested block and send a MsgBlockResponse.
	case MsgBlockResponse:
		nm.logger.Debug("NETWORK_MANAGER: Received Block Response. (Conceptual: Process requested block)")
		// Add to blockchain or pass to sync manager.
	case MsgBlockVote:
		nm.logger.Debug("NETWORK_MANAGER: Received Block Vote. (Conceptual: Pass to consensus mechanism)")
		// Pass to higher-level consensus logic.
	case MsgAILog:
		nm.logger.Debug("NETWORK_MANAGER: Received AI Log message. (Conceptual: Store/process AI audit data)")
		// EmPower1 specific, for transparency and auditability of AI decisions.
	case MsgWealthUpdate:
		nm.logger.Debug("NETWORK_MANAGER: Received Wealth Update message. (Conceptual: Process and verify wealth updates)")
		// EmPower1 specific, for synchronizing AI/ML wealth assessments.
	default:
		nm.logger.Warnf("NETWORK_MANAGER_WARN: Received unhandled message type %s from %s.", msg.Type.String(), p.Address())
	}
}

// --- Internal Helper Methods for NetworkManager ---

// discoverAndConnect attempts to find and connect to new peers from the knownAddresses list.
func (nm *NetworkManager) discoverAndConnect() {
	nm.logger.Debug("NETWORK_MANAGER: Running peer discovery...")

	nm.mu.RLock() // Read-lock for iterating knownAddresses and activePeers
	currentPeersCount := len(nm.activePeers)
	addressesToTry := make([]string, 0, len(nm.knownAddresses))
	for addr := range nm.knownAddresses {
		if nm.activePeers[addr] == nil { // Only try to connect if not already active
			addressesToTry = append(addressesToTry, addr)
		}
	}
	nm.mu.RUnlock()

	// Shuffle addresses to randomize connection attempts
	// rand.Shuffle(len(addressesToTry), func(i, j int) { addressesToTry[i], addressesToTry[j] = addressesToTry[j], addressesToTry[i] })
	
	// Try to connect to a few new peers if below desired peer count
	connectionsMade := 0
	for _, addr := range addressesToTry {
		if connectionsMade >= maxDesiredPeers-currentPeersCount {
			break // Stop if we've made enough connections
		}
		if currentPeersCount+connectionsMade >= maxDesiredPeers {
			break // Don't exceed max desired peers
		}

		nodeID, _ := nm.knownAddresses[addr] // Get associated ID (might be nil if only address was known)
		
		// Connect asynchronously to avoid blocking the loop
		go func(address string, id []byte) {
			nm.logger.Debugf("NETWORK_MANAGER: Attempting connection to %s (ID: %x)...", address, id)
			_, err := nm.server.Connect(address, id) // s.Connect returns *Peer or error.
			if err != nil {
				nm.logger.Debugf("NETWORK_MANAGER: Failed to connect to %s: %v", address, err)
				// If error indicates permanent failure (e.g., unreachable), might remove from knownAddresses (V2+).
			} else {
				// Success is handled by handlePeerConnected callback from Server
				nm.logger.Debugf("NETWORK_MANAGER: Successfully initiated connection to %s", address)
			}
		}(addr, nodeID)
		connectionsMade++
		// Add a small delay to avoid hammering
		time.Sleep(50 * time.Millisecond)
	}
	if connectionsMade > 0 {
		nm.logger.Printf("NETWORK_MANAGER: Initiated %d new connection attempts.", connectionsMade)
	}
}

// pruneStaleConnections checks active peers for liveness and disconnects stale ones.
func (nm *NetworkManager) pruneStaleConnections() {
	nm.logger.Debug("NETWORK_MANAGER: Running connection maintenance...")
	
	nm.mu.RLock()
	peersToCheck := make([]*Peer, 0, len(nm.activePeers))
	for _, p := range nm.activePeers {
		peersToCheck = append(peersToCheck, p)
	}
	nm.mu.RUnlock()

	for _, p := range peersToCheck {
		if time.Since(p.GetLastActivity()) > 3*peerLivenessCheckInterval { // If no activity for a while
			nm.logger.Printf("NETWORK_MANAGER: Peer %s (ID: %x) appears stale. Last activity: %s. Disconnecting.", 
				p.Address(), p.ID(), p.GetLastActivity().Format(time.RFC3339))
			// Remove peer (this also closes connection)
			if err := nm.server.removePeer(p); err != nil { // Assuming Server has a public removePeer method
				nm.logger.Errorf("NETWORK_MANAGER: Error removing stale peer %s: %v", p.Address(), err)
			}
		}
	}
	// TODO: If activePeers count drops below minDesiredPeers, trigger immediate discovery.
}

// gossipBlock implements simple block propagation logic.
// It sends the block to all active peers except the original sender.
func (nm *NetworkManager) gossipBlock(block *core.Block, originalSender *Peer) {
	nm.logger.Debugf("NETWORK_MANAGER: Gossiping block %x from %x.", block.Hash, originalSender.ID())
	// In a production system, this would involve:
	// - Checking if peer already knows this block (bloom filter or inv/getdata logic).
	// - Prioritizing sending to certain peers (e.g., by latency, bandwidth).
	// - Not sending back to the original sender.
	nm.server.BroadcastMessage(NewMessage(MsgNewBlockProposal, nm.selfNodeID, block.SerializeToBytes()), originalSender) // Assuming Block.SerializeToBytes() exists
}

// gossipTransaction implements simple transaction propagation logic.
// It sends the transaction to all active peers except the original sender.
func (nm *NetworkManager) gossipTransaction(tx *core.Transaction, originalSender *Peer) {
	nm.logger.Debugf("NETWORK_MANAGER: Gossiping tx %x from %x.", tx.ID, originalSender.ID())
	// Similar considerations as gossipBlock for efficiency and avoiding redundant sends.
	nm.server.BroadcastMessage(NewMessage(MsgNewTransaction, nm.selfNodeID, tx.Serialize()), originalSender) // Assuming Transaction.Serialize() exists
}

// --- Public Interface for higher-level services ---
// These methods allow ConsensusEngine and Mempool to interact with the network.

// SendBlockToPeer sends a specific block to a specific peer.
// Useful for block requests/responses during sync.
func (nm *NetworkManager) SendBlockToPeer(block *core.Block, peerID []byte) error {
    peer := nm.server.GetPeerByID(peerID) // Assuming Server has a GetPeerByID method
    if peer == nil {
        return fmt.Errorf("%w: peer %x not found to send block", ErrPeerNotFound, peerID)
    }
    blockBytes, err := block.SerializeToBytes() // Assuming Block.SerializeToBytes()
    if err != nil {
        return fmt.Errorf("%w: failed to serialize block for sending: %v", ErrMessageSerialization, err)
    }
    msg := NewMessage(MsgBlockResponse, nm.selfNodeID, blockBytes)
    return nm.server.sendMessage(peer, msg) // Use internal Server.sendMessage
}

// RequestBlockFromPeer requests a specific block from a peer.
func (nm *NetworkManager) RequestBlockFromPeer(peerID []byte, blockHash []byte, height int64) error {
    peer := nm.server.GetPeerByID(peerID)
    if peer == nil {
        return fmt.Errorf("%w: peer %x not found to request block", ErrPeerNotFound, peerID)
    }
    payload := BlockRequestPayload{BlockHash: blockHash, Height: height}
    payloadBytes, err := EncodePayload(payload)
    if err != nil {
        return fmt.Errorf("%w: failed to encode block request payload: %v", ErrPayloadEncoding, err)
    }
    msg := NewMessage(MsgBlockRequest, nm.selfNodeID, payloadBytes)
    return nm.server.sendMessage(peer, msg)
}

// SendVoteToPeer sends a block vote message to a specific peer.
func (nm *NetworkManager) SendVoteToPeer(vote *BlockVotePayload, peerID []byte) error {
    peer := nm.server.GetPeerByID(peerID)
    if peer == nil {
        return fmt.Errorf("%w: peer %x not found to send vote", ErrPeerNotFound, peerID)
    }
    payloadBytes, err := EncodePayload(vote)
    if err != nil {
        return fmt.Errorf("%w: failed to encode vote payload: %v", ErrPayloadEncoding, err)
    }
    msg := NewMessage(MsgBlockVote, nm.selfNodeID, payloadBytes)
    return nm.server.sendMessage(peer, msg)
}

// SendTransactionToPeer sends a specific transaction to a specific peer.
func (nm *NetworkManager) SendTransactionToPeer(tx *core.Transaction, peerID []byte) error {
    peer := nm.server.GetPeerByID(peerID)
    if peer == nil {
        return fmt.Errorf("%w: peer %x not found to send transaction", ErrPeerNotFound, peerID)
    }
    txBytes, err := tx.Serialize() // Assuming Transaction.Serialize()
    if err != nil {
        return fmt.Errorf("%w: failed to serialize transaction for sending: %v", ErrMessageSerialization, err)
    }
    msg := NewMessage(MsgNewTransaction, nm.selfNodeID, txBytes)
    return nm.server.sendMessage(peer, msg)
}