package network

import (
	"bytes"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output
	"sync"
	"time"

	"empower1.com/core/core" // Assuming 'core' is the package alias for empower1.com/core/core
)

// Define custom errors for the Network package.
var (
	ErrNetworkAlreadyRunning = errors.New("network is already running")
	ErrNetworkNotRunning     = errors.New("network is not running")
	ErrBroadcastFailed       = errors.New("failed to broadcast message")
	ErrInvalidPeer           = errors.New("invalid peer")
)

// Peer represents a conceptual peer in the network.
// In a real P2P network, this would include connection details (IP, port, ID).
type Peer struct {
	ID      []byte // Cryptographic ID (e.g., public key hash)
	Address string // Conceptual network address (e.g., "127.0.0.1:8080")
	// For simulation, we'll use an internal channel for direct communication.
	incomingMsgs chan interface{} // Channel for direct simulation of messages to this peer
}

// Network manages the peer-to-peer communication.
// For V1, this is a simulated in-memory network.
type Network struct {
	mu           sync.RWMutex      // Mutex for concurrent access to peer map
	peers        map[string]*Peer  // Map of connected peers by hex ID
	isRunning    bool              // Flag to track network status
	stopChan     chan struct{}     // Channel to signal network shutdown
	wg           sync.WaitGroup    // WaitGroup for goroutines
	logger       *log.Logger       // Dedicated logger for the Network instance

	// Channels for other services to receive blocks/transactions from the network
	blockChan    chan *core.Block
	txChan       chan *core.Transaction

	// This node's own ID for broadcasting
	selfID       []byte
}

// NewNetwork creates a new simulated P2P Network instance.
// It sets up internal channels and prepares for peer management.
// selfID is the ID of the node using this network instance (e.g., the local blockchain node).
func NewNetwork(selfID []byte, blockBufferSize, txBufferSize int) (*Network, error) {
	if len(selfID) == 0 {
		return nil, fmt.Errorf("%w: self ID cannot be empty", ErrNetworkAlreadyRunning) // Reusing error
	}
	if blockBufferSize <= 0 || txBufferSize <= 0 {
		return nil, fmt.Errorf("buffer sizes must be positive")
	}

	logger := log.New(os.Stdout, "NETWORK: ", log.Ldate|log.Ltime|log.Lshortfile)
	
	net := &Network{
		peers:        make(map[string]*Peer),
		isRunning:    false,
		stopChan:     make(chan struct{}),
		wg:           sync.WaitGroup{},
		logger:       logger,
		blockChan:    make(chan *core.Block, blockBufferSize),
		txChan:       make(chan *core.Transaction, txBufferSize),
		selfID:       selfID,
	}
	net.logger.Println("Network initialized for node:", hex.EncodeToString(selfID))
	return net, nil
}

// Start initiates the network's operation.
// It starts internal goroutines for processing messages.
func (net *Network) Start() error {
	net.mu.Lock()
	defer net.mu.Unlock()

	if net.isRunning {
		return ErrNetworkAlreadyRunning
	}
	net.isRunning = true

	// In a real network, this would start the listening server for incoming connections
	// and goroutines for maintaining connections to peers.
	// For simulation, we will conceptually "process" internal peer message channels.

	net.logger.Println("Network started.")
	return nil
}

// Stop gracefully shuts down the network.
func (net *Network) Stop() error {
	net.mu.Lock()
	defer net.mu.Unlock()

	if !net.isRunning {
		return ErrNetworkNotRunning
	}
	net.isRunning = false

	close(net.stopChan)     // Signal all internal goroutines to stop
	net.wg.Wait()           // Wait for all goroutines to finish

	// Close outgoing channels (if any) and incoming public channels
	close(net.blockChan)
	close(net.txChan)

	net.logger.Println("Network stopped.")
	return nil
}

// ConnectPeer conceptually adds a new peer to the network.
// In a real network, this would establish a persistent connection.
func (net *Network) ConnectPeer(peerID []byte, peerAddress string) (*Peer, error) {
	net.mu.Lock()
	defer net.mu.Unlock()

	if len(peerID) == 0 {
		return nil, ErrInvalidPeer
	}
	peerIDHex := hex.EncodeToString(peerID)

	if _, exists := net.peers[peerIDHex]; exists {
		net.logger.Debugf("NETWORK: Peer %s already connected.", peerIDHex)
		return net.peers[peerIDHex], nil
	}
	
	// Create a new conceptual peer with an incoming message channel for simulation
	newPeer := &Peer{
		ID:           peerID,
		Address:      peerAddress,
		incomingMsgs: make(chan interface{}, 100), // Buffered channel for simulation
	}
	net.peers[peerIDHex] = newPeer
	net.logger.Printf("NETWORK: Connected to new peer: %s", peerAddress)
	return newPeer, nil
}

// DisconnectPeer conceptually removes a peer from the network.
func (net *Network) DisconnectPeer(peerID []byte) error {
	net.mu.Lock()
	defer net.mu.Unlock()

	peerIDHex := hex.EncodeToString(peerID)
	if peer, exists := net.peers[peerIDHex]; exists {
		delete(net.peers, peerIDHex)
		close(peer.incomingMsgs) // Close the peer's channel
		net.logger.Printf("NETWORK: Disconnected from peer: %s", peer.Address)
		return nil
	}
	return fmt.Errorf("peer %s not found", peerIDHex)
}


// BroadcastBlock simulates broadcasting a block to all connected peers.
// For simulation, it sends the block to each peer's conceptual incoming message channel.
// This implements the `SimulatedNetwork` interface.
func (net *Network) BroadcastBlock(block *core.Block) error {
	net.mu.RLock() // Acquire read lock for iterating peers
	defer net.mu.RUnlock()

	if !net.isRunning {
		return ErrNetworkNotRunning
	}
	if block == nil {
		return fmt.Errorf("%w: cannot broadcast nil block", ErrBroadcastFailed)
	}

	blockHashHex := hex.EncodeToString(block.Hash)
	net.logger.Printf("NETWORK: Broadcasting block #%d (%s) to %d peers.", block.Height, blockHashHex, len(net.peers))

	// Simulate sending to each peer. In a real system, this would be over actual network connections.
	// Use a goroutine to avoid blocking if a peer's channel is full.
	for _, peer := range net.peers {
		if bytes.Equal(peer.ID, net.selfID) { // Don't send to self
			continue
		}
		// Try to send without blocking. If channel is full, log a warning (conceptual congestion).
		select {
		case peer.incomingMsgs <- block:
			net.logger.Debugf("NETWORK: Sent block %s to peer %s", blockHashHex, hex.EncodeToString(peer.ID))
		default:
			net.logger.Warnf("NETWORK_WARN: Peer %s's channel full, failed to send block %s.", hex.EncodeToString(peer.ID), blockHashHex)
		}
	}
	return nil
}

// BroadcastTransaction simulates broadcasting a transaction to all connected peers.
func (net *Network) BroadcastTransaction(tx *core.Transaction) error {
	net.mu.RLock()
	defer net.mu.RUnlock()

	if !net.isRunning {
		return ErrNetworkNotRunning
	}
	if tx == nil {
		return fmt.Errorf("%w: cannot broadcast nil transaction", ErrBroadcastFailed)
	}

	txIDHex := hex.EncodeToString(tx.ID)
	net.logger.Printf("NETWORK: Broadcasting transaction %s to %d peers.", txIDHex, len(net.peers))

	for _, peer := range net.peers {
		if bytes.Equal(peer.ID, net.selfID) { // Don't send to self
			continue
		}
		select {
		case peer.incomingMsgs <- tx:
			net.logger.Debugf("NETWORK: Sent transaction %s to peer %s", txIDHex, hex.EncodeToString(peer.ID))
		default:
			net.logger.Warnf("NETWORK_WARN: Peer %s's channel full, failed to send transaction %s.", hex.EncodeToString(peer.ID), txIDHex)
		}
	}
	return nil
}

// ReceiveBlocks returns a channel for receiving incoming blocks.
// This is the public interface for the ConsensusEngine or Blockchain service to consume blocks.
func (net *Network) ReceiveBlocks() <-chan *core.Block {
	return net.blockChan
}

// ReceiveTransactions returns a channel for receiving incoming transactions.
// This is the public interface for the Mempool service to consume transactions.
func (net *Network) ReceiveTransactions() <-chan *core.Transaction {
	return net.txChan
}

// conceptualPeerMessageProcessor simulates processing messages received by a peer.
// In a real network, this would be the inbound message handler for each connection.
// For simulation, it reads from peer.incomingMsgs and routes to the appropriate public channel.
func (net *Network) conceptualPeerMessageProcessor(p *Peer) {
	defer net.wg.Done()
	net.logger.Printf("NETWORK: Peer message processor started for %s", hex.EncodeToString(p.ID))

	for {
		select {
		case <-net.stopChan:
			net.logger.Debugf("NETWORK: Peer message processor for %s received stop signal.", hex.EncodeToString(p.ID))
			return
		case msg, ok := <-p.incomingMsgs:
			if !ok { // Channel closed
				net.logger.Debugf("NETWORK: Peer %s incoming channel closed.", hex.EncodeToString(p.ID))
				return
			}
			switch v := msg.(type) {
			case *core.Block:
				// Try to send block to blockChan. If full, it means the downstream service is slow.
				select {
				case net.blockChan <- v:
					net.logger.Debugf("NETWORK: Received and routed block %x from %s", v.Hash, hex.EncodeToString(p.ID))
				default:
					net.logger.Warnf("NETWORK_WARN: Block channel full, dropping block %x from %s.", v.Hash, hex.EncodeToString(p.ID))
				}
			case *core.Transaction:
				// Try to send transaction to txChan.
				select {
				case net.txChan <- v:
					net.logger.Debugf("NETWORK: Received and routed tx %x from %s", v.ID, hex.EncodeToString(p.ID))
				default:
					net.logger.Warnf("NETWORK_WARN: Tx channel full, dropping tx %x from %s.", v.ID, hex.EncodeToString(p.ID))
				}
			default:
				net.logger.Warnf("NETWORK_WARN: Received unknown message type from %s.", hex.EncodeToString(p.ID))
			}
		}
	}
}

// SimulatePeerDiscovery conceptually adds a list of initial peers to the network.
// This would be replaced by actual peer discovery mechanisms (e.g., DNS seeds, Kademlia DHT).
func (net *Network) SimulatePeerDiscovery(peerIDs [][]byte, peerAddresses []string) {
    net.mu.Lock()
    defer net.mu.Unlock()

    for i, id := range peerIDs {
        if bytes.Equal(id, net.selfID) { // Don't connect to self
            continue
        }
        addr := peerAddresses[i]
        // ConnectPeer handles adding, but here we just need to create the peer and its channel
        if _, exists := net.peers[hex.EncodeToString(id)]; !exists {
            newPeer := &Peer{
                ID:           id,
                Address:      addr,
                incomingMsgs: make(chan interface{}, 100), // Buffered channel
            }
            net.peers[hex.EncodeToString(id)] = newPeer
            net.wg.Add(1) // Add for this new conceptual peer processor
            go net.conceptualPeerMessageProcessor(newPeer)
            net.logger.Printf("NETWORK: Discovered and simulated connection to peer: %s (%s)", hex.EncodeToString(id), addr)
        }
    }
}