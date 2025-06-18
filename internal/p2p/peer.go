package p2p

import (
	"fmt"
	"net"
	"sync"
	"time" // Added for potential last activity timestamp for peer health

	"empower1.com/core/core" // Assuming this has relevant types if needed for peer ID
)

// Peer represents a connected node in the EmPower1 P2P network.
// It encapsulates the network connection and state relevant to that peer.
type Peer struct {
	conn        net.Conn           // The underlying network connection
	id          []byte             // Cryptographic ID of the peer (e.g., public key hash of the remote node)
	address     string             // Network address of the peer (e.g., "127.0.0.1:8080")
	mu          sync.RWMutex       // Mutex for protecting concurrent access to mutable peer state
	knownPeers  map[string]bool    // Tracks peers known by *this* peer (addresses string)
	isInitiator bool               // True if our node initiated the connection to this peer
	lastActivity time.Time         // Timestamp of the last received message from this peer (for liveness checks)
	// V2+: ReputationScore float64 // Reputation of this peer (e.g., for routing or trust)
	// V2+: Latency        time.Duration // Measured network latency to this peer
	// V2+: ProtocolVersion string     // The protocol version supported by this peer
}

// NewPeer creates a new Peer instance from an established network connection.
// It initializes the peer's basic state, adhering to "Know Your Core, Keep it Clear".
func NewPeer(conn net.Conn, isInitiator bool, peerID []byte) (*Peer, error) {
	if conn == nil {
		return nil, errors.New("net.Conn cannot be nil for new peer")
	}
	if len(peerID) == 0 {
		return nil, errors.New("peer ID cannot be empty for new peer")
	}

	return &Peer{
		conn:        conn,
		id:          peerID,
		address:     conn.RemoteAddr().String(),
		knownPeers:  make(map[string]bool),
		isInitiator: isInitiator,
		lastActivity: time.Now(), // Initialize with current time
	}, nil
}

// ID returns the cryptographic ID of the peer.
func (p *Peer) ID() []byte {
	return p.id
}

// Address returns the network address of the peer.
func (p *Peer) Address() string {
	return p.address
}

// Conn returns the underlying net.Conn of the peer.
// Direct access to the connection should be used with care.
func (p *Peer) Conn() net.Conn {
	return p.conn
}

// AddKnownPeer adds a peer's address to the set of known peers for this peer.
// This is part of the peer discovery and gossip mechanism.
func (p *Peer) AddKnownPeer(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.knownPeers[address]; !exists {
		p.knownPeers[address] = true
		// Potentially log this addition
	}
}

// KnownPeers returns a copy of the slice of known peer addresses.
// Provides safe, concurrent access to the list of known peers.
func (p *Peer) KnownPeers() []string {
	p.mu.RLock() // Use RLock for read access
	defer p.mu.RUnlock()
	peers := make([]string, 0, len(p.knownPeers))
	for addr := range p.knownPeers {
		peers = append(peers, addr)
	}
	return peers
}

// HasKnownPeer checks if a peer address is in the known list.
func (p *Peer) HasKnownPeer(address string) bool {
	p.mu.RLock() // Use RLock for read access
	defer p.mu.RUnlock()
	_, exists := p.knownPeers[address]
	return exists
}

// IsInitiator returns true if this peer instance initiated the connection to the remote peer.
// Useful for distinguishing inbound vs. outbound connections.
func (p *Peer) IsInitiator() bool {
	return p.isInitiator
}

// UpdateLastActivity updates the timestamp of the last communication with this peer.
// Critical for liveness tracking and connection management.
func (p *Peer) UpdateLastActivity() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastActivity = time.Now()
}

// GetLastActivity returns the timestamp of the last communication.
func (p *Peer) GetLastActivity() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastActivity
}

// Close closes the underlying network connection to the peer.
// This should be called when a peer is disconnected.
func (p *Peer) Close() error {
	p.mu.Lock() // Lock to prevent other operations while closing
	defer p.mu.Unlock()
	// Check if connection is already closed to avoid errors on double close
	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil // Mark as nil after closing
		return err
	}
	return nil // Already closed or never established
}