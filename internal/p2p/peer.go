package p2p

import (
	"net"
	"sync"
)

// Peer represents a connected node in the network.
type Peer struct {
	conn        net.Conn
	address     string
	mu          sync.Mutex
	knownPeers  map[string]bool // Tracks peers known by this peer
	isInitiator bool            // True if this node initiated the connection
}

// NewPeer creates a new Peer instance.
func NewPeer(conn net.Conn, isInitiator bool) *Peer {
	return &Peer{
		conn:        conn,
		address:     conn.RemoteAddr().String(),
		knownPeers:  make(map[string]bool),
		isInitiator: isInitiator,
	}
}

// Address returns the network address of the peer.
func (p *Peer) Address() string {
	return p.address
}

// Conn returns the underlying net.Conn of the peer.
func (p *Peer) Conn() net.Conn {
	return p.conn
}

// AddKnownPeer adds a peer to the set of known peers for this peer.
func (p *Peer) AddKnownPeer(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.knownPeers[address] = true
}

// KnownPeers returns a slice of known peer addresses.
func (p *Peer) KnownPeers() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	peers := make([]string, 0, len(p.knownPeers))
	for addr := range p.knownPeers {
		peers = append(peers, addr)
	}
	return peers
}

// HasKnownPeer checks if a peer address is in the known list.
func (p *Peer) HasKnownPeer(address string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, exists := p.knownPeers[address]
	return exists
}

// IsInitiator returns true if this peer instance initiated the connection.
func (p *Peer) IsInitiator() bool {
	return p.isInitiator
}

// Close closes the connection to the peer.
func (p *Peer) Close() error {
	return p.conn.Close()
}
