package p2p

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob" // Ensure gob is imported for serialization
	"errors"
	"fmt"
	"io"
	"log" // Using standard log package for better control
	"net"
	"strings"
	"sync"
	"time"

	"empower1.com/core/core" // Assuming 'core' is the package alias for empower1.com/core/core
)

// --- Custom Errors for P2P Server ---
var (
	ErrServerAlreadyRunning = errors.New("p2p server is already running")
	ErrServerNotRunning     = errors.New("p2p server is not running")
	ErrFailedToListen       = errors.New("failed to listen on address")
	ErrConnectionAccept     = errors.New("failed to accept connection")
	ErrHandshakeFailed      = errors.New("p2p handshake failed")
	ErrPeerNotFound         = errors.New("peer not found in server's map")
	ErrSendMessageFailed    = errors.New("failed to send message to peer")
	ErrReceiveMessageFailed = errors.New("failed to receive message from peer")
	ErrPeerNotHandshaked    = errors.New("message received from non-handshaked peer")
	ErrConnectTimeout       = errors.New("connection timeout")
)

const (
	protocolVersion = "empower1/0.1" // Current protocol version for EmPower1
	readBufSize     = 4096           // Buffer size for reading messages
	dialTimeout     = 5 * time.Second // Timeout for initiating outbound connections
)

// Server manages incoming peer connections, maintains peer list, and facilitates message broadcasting.
// This is the core P2P networking component for EmPower1 Blockchain.
type Server struct {
	listenAddress string               // Address this server listens on (e.g., ":8080")
	listener      net.Listener         // The underlying TCP listener
	peers         map[string]*Peer     // Map of connected peers, keyed by their network address string
	mu            sync.RWMutex         // Mutex for protecting 'peers' map and 'isRunning' status
	quit          chan struct{}        // Channel to signal server shutdown
	wg            sync.WaitGroup       // WaitGroup for managing all spawned goroutines
	logger        *log.Logger          // Dedicated logger for the P2P Server

	// Callbacks: Used by higher-level services (e.g., ConsensusEngine) to process network events.
	OnPeerConnected    func(p *Peer)
	OnPeerDisconnected func(p *Peer)
	OnMessage          func(p *Peer, msg *Message) // Called when a valid message is received
	// OnUnhandledMessage (optional): For messages that are valid but not handled by specific types.
}

// NewServer creates a new P2P Server instance.
// It initializes core data structures and sets up logging.
func NewServer(listenAddress string) (*Server, error) {
	if listenAddress == "" {
		return nil, fmt.Errorf("%w: listen address cannot be empty", ErrServerInit)
	}
	logger := log.New(os.Stdout, "P2P_SERVER: ", log.Ldate|log.Ltime|log.Lshortfile)
	s := &Server{
		listenAddress: listenAddress,
		peers:         make(map[string]*Peer),
		quit:          make(chan struct{}),
		logger:        logger,
	}
	s.logger.Printf("P2P Server initialized for address: %s", listenAddress)
	return s, nil
}

// Start initializes and starts the P2P server's listener and background goroutines.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil { // Already started
		return ErrServerAlreadyRunning
	}

	ln, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToListen, err)
	}
	s.listener = ln
	
	s.wg.Add(1) // Goroutine for acceptLoop
	go s.acceptLoop()

	s.logger.Printf("P2P server listening on %s", s.ListenAddress())
	return nil
}

// Stop gracefully shuts down the P2P server and all connected peers.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil { // Not running or already stopped
		return
	}
	
	close(s.quit) // Signal all goroutines to stop
	s.listener.Close() // Close the listener to unblock acceptLoop
	s.listener = nil // Clear listener reference

	// Close all peer connections
	for addr, peer := range s.peers {
		if err := peer.Close(); err != nil {
			s.logger.Errorf("Error closing peer %s connection: %v", addr, err)
		}
		delete(s.peers, addr) // Remove from map
	}
	s.peers = make(map[string]*Peer) // Reset map

	s.wg.Wait() // Wait for all goroutines (acceptLoop, readLoops) to finish
	s.logger.Println("P2P Server stopped.")
}

// acceptLoop continuously accepts new incoming connections.
func (s *Server) acceptLoop() {
	defer s.wg.Done()
	s.logger.Debug("Accept loop started.")

	for {
		select {
		case <-s.quit: // Check for quit signal first
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				// Check if the error is due to listener being closed (normal shutdown)
				select {
				case <-s.quit:
					return
				default:
					// Log other transient accept errors
					s.logger.Errorf("%w: %v", ErrConnectionAccept, err)
					if ne, ok := err.(net.Error); ok && ne.Temporary() {
						time.Sleep(100 * time.Millisecond) // Avoid busy-looping on temporary errors
					}
				}
				continue // Continue accept loop
			}
			// Handle each new connection in a goroutine to not block the accept loop
			s.wg.Add(1) // Add for handleConnection goroutine
			go s.handleConnection(conn, false) // `false` because this is an inbound connection
		}
	}
}

// handleConnection performs the handshake and starts the read loop for a new connection.
func (s *Server) handleConnection(conn net.Conn, initiator bool) {
    defer s.wg.Done() // Ensure WaitGroup is decremented

    peerID := []byte(conn.RemoteAddr().String()) // Placeholder: In real P2P, get remote node's actual ID during handshake
    peer, err := NewPeer(conn, initiator, peerID)
    if err != nil {
        s.logger.Errorf("Failed to create peer for %s: %v", conn.RemoteAddr().String(), err)
        conn.Close()
        return
    }

    s.logger.Printf("New connection from %s (Initiator: %t)", peer.Address(), initiator)

    // Handshake process (send/receive HELLO messages)
    if initiator { // We initiated the connection, so we send HELLO first
        if err := s.sendHello(peer); err != nil {
            s.logger.Errorf("Handshake failed with %s: failed to send HELLO: %v", peer.Address(), err)
            peer.Close()
            return
        }
        // Then read their HELLO reply
        if _, err := s.readAndProcessHello(peer); err != nil { // This is where peer is potentially added
             s.logger.Errorf("Handshake failed with %s: failed to receive/process HELLO reply: %v", peer.Address(), err)
             peer.Close()
             return
        }
    } else { // They initiated, so we expect their HELLO first
        if _, err := s.readAndProcessHello(peer); err != nil { // This is where peer is potentially added
            s.logger.Errorf("Handshake failed with %s: failed to receive/process HELLO: %v", peer.Address(), err)
            peer.Close()
            return
        }
        // Then send our HELLO back (acknowledgment)
        if err := s.sendHello(peer); err != nil {
            s.logger.Errorf("Handshake failed with %s: failed to send HELLO reply: %v", peer.Address(), err)
            peer.Close()
            return
            // Note: If this fails, the peer was already added in readAndProcessHello, so remove it.
        }
    }
    
    // After successful handshake, the peer is added to our map by readAndProcessHello
    // and its read loop is started.

    // Start the read loop for this peer
    s.wg.Add(1) // Goroutine for readLoop
    go s.readLoop(peer)
}

// readAndProcessHello reads an incoming HELLO message and processes it.
// This function is crucial for initial peer discovery and connection establishment.
// It also adds the peer to the server's peer map if the handshake is valid.
// Returns the received HelloPayload (for information) or an error.
func (s *Server) readAndProcessHello(p *Peer) (*HelloPayload, error) {
    // Read message length and data
    msgData, err := s.readMessageData(p.Conn())
    if err != nil {
        return nil, fmt.Errorf("%w: failed to read HELLO message data from %s: %v", ErrHandshakeFailed, p.Address(), err)
    }

    msg, err := DeserializeMessage(msgData)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to deserialize HELLO message from %s: %v", ErrHandshakeFailed, p.Address(), err)
    }

    if msg.Type != MsgHello {
        return nil, fmt.Errorf("%w: expected MsgHello, got %s from %s", ErrHandshakeFailed, msg.Type.String(), p.Address())
    }

    var helloPayload HelloPayload
    // Use DecodePayload for cleaner handling
    if err := DecodePayload(msg.Payload, &helloPayload); err != nil {
        return nil, fmt.Errorf("%w: failed to decode HELLO payload from %s: %v", ErrHandshakeFailed, p.Address(), err)
    }

    // --- Basic HELLO Payload Validation ---
    if !strings.HasPrefix(helloPayload.Version, "empower1/") {
        return nil, fmt.Errorf("%w: incompatible protocol version '%s' from %s", ErrHandshakeFailed, helloPayload.Version, p.Address())
    }
    if len(helloPayload.NodeID) == 0 { // Peer's ID must be provided
        return nil, fmt.Errorf("%w: HELLO missing NodeID from %s", ErrHandshakeFailed, p.Address())
    }
    // TODO: Add more robust validation, e.g., signature of hello message (V2+)

    // Update peer's ID from handshake (actual ID, not just remote address)
    p.id = helloPayload.NodeID
    s.logger.Printf("Handshake: Received HELLO from Peer ID: %x, Version: %s, ListenAddr: %s, KnownPeers: %d", 
        p.ID(), helloPayload.Version, helloPayload.ListenAddr, len(helloPayload.KnownPeers))

    // Add peer to server's list AFTER successful handshake
    s.addPeer(p)

    // Add peers from their hello message to this peer's known list
    // And potentially to server's global discovery queue.
    if helloPayload.ListenAddr != "" {
        p.AddKnownPeer(helloPayload.ListenAddr)
        // Add to server's global list for potential connection attempts if it's a new peer
        s.tryConnectToPeer(helloPayload.ListenAddr, helloPayload.NodeID) // Conceptual: see below
    }
    for _, addr := range helloPayload.KnownPeers {
        p.AddKnownPeer(addr)
        s.tryConnectToPeer(addr, nil) // NodeID might not be known yet from peer list, if nil, it means lazy discover
    }

    return &helloPayload, nil
}


// sendHello sends a HELLO message to a specific peer.
func (s *Server) sendHello(p *Peer) error {
    // Current chain height from blockchain (Conceptual, passed from ConsensusEngine)
    currentChainHeight := int64(0) // Placeholder for actual height
    // Own Node ID (Conceptual, passed from main app/node context)
    ownNodeID := []byte("self_node_id_placeholder") 

    helloPayload := HelloPayload{
        Version:      protocolVersion,
        ListenAddr:   s.ListenAddress(),
        NodeID:       ownNodeID,
        KnownPeers:   s.KnownPeers(), // Our currently connected peer addresses
        CurrentHeight: currentChainHeight,
    }
    payloadBytes, err := EncodePayload(helloPayload)
    if err != nil {
        return fmt.Errorf("%w: failed to serialize HELLO payload: %v", ErrPayloadEncoding, err)
    }
    msg := NewMessage(MsgHello, ownNodeID, payloadBytes) // Use NewMessage constructor
    return s.sendMessage(p, msg) // Use internal sendMessage
}

// SendPeerList sends a list of peer addresses to the specified peer.
func (s *Server) SendPeerList(p *Peer, peersToSend []string) error {
    ownNodeID := []byte("self_node_id_placeholder")
    peerListPayload := PeerListPayload{Peers: peersToSend}
    payloadBytes, err := EncodePayload(peerListPayload)
    if err != nil {
        return fmt.Errorf("%w: failed to serialize PEER_LIST payload: %v", ErrPayloadEncoding, err)
    }
    msg := NewMessage(MsgPeerList, ownNodeID, payloadBytes)
    return s.sendMessage(p, msg)
}

// sendRequestPeerList sends a request for known peers to the specified peer.
func (s *Server) sendRequestPeerList(p *Peer) error {
    ownNodeID := []byte("self_node_id_placeholder")
    msg := NewMessage(MsgRequestPeerList, ownNodeID, []byte{}) // Empty payload for request
    return s.sendMessage(p, msg)
}

// SendMessage (internal) handles message framing and sending to a specific peer.
// This is a private helper, all public send methods should wrap this.
func (s *Server) sendMessage(p *Peer, msg *Message) error {
    data, err := msg.Serialize()
    if err != nil {
        return fmt.Errorf("%w: failed to serialize message for peer %s: %v", ErrMessageSerialization, p.Address(), err)
    }

    // Framing: 4 bytes for message length (uint32), then message data
    msgLen := uint32(len(data))
    lenBuf := make([]byte, 4)
    binary.BigEndian.PutUint32(lenBuf, msgLen)

    // Use a buffered writer for efficiency
    writer := bufio.NewWriter(p.Conn())

    if _, err := writer.Write(lenBuf); err != nil {
        s.logger.Errorf("Failed to send message length to peer %s: %v", p.Address(), err)
        s.removePeer(p) // Remove peer on write error
        return fmt.Errorf("%w: %v", ErrSendMessageFailed, err)
    }

    if _, err := writer.Write(data); err != nil {
        s.logger.Errorf("Failed to send message data to peer %s: %v", p.Address(), err)
        s.removePeer(p)
        return fmt.Errorf("%w: %v", ErrSendMessageFailed, err)
    }
    // Flush the buffer to ensure data is sent over the network
    if err := writer.Flush(); err != nil {
        s.logger.Errorf("Failed to flush message to peer %s: %v", p.Address(), err)
        s.removePeer(p)
        return fmt.Errorf("%w: %v", ErrSendMessageFailed, err)
    }

    s.logger.Debugf("Sent %s to %s (Length: %d)", msg.Type.String(), p.Address(), len(data))
    return nil
}

// BroadcastMessage sends a message to all connected peers except the excludePeer.
// This is a public method for network-wide message propagation.
func (s *Server) BroadcastMessage(msg *Message, excludePeer *Peer) {
	s.mu.RLock() // Use RLock for reading peers map
	peersToBroadcast := make([]*Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		if excludePeer != nil && peer.Address() == excludePeer.Address() {
			continue // Skip the excluded peer
		}
		peersToBroadcast = append(peersToBroadcast, peer)
	}
	s.mu.RUnlock() // Release RLock before calling sendMessage to avoid deadlock (sendMessage acquires Server.mu.Lock indirectly via removePeer)

	s.logger.Printf("Broadcasting %s to %d peers (excluding %s).", msg.Type.String(), len(peersToBroadcast), excludePeer.Address())

	for _, peer := range peersToBroadcast {
		// Send messages in goroutines to avoid blocking the broadcast loop if a peer is slow/stuck.
		// Error handling for sending is now within sendMessage.
		go func(p *Peer, m *Message) {
			if err := s.sendMessage(p, m); err != nil {
				s.logger.Errorf("Error broadcasting %s to peer %s: %v", m.Type.String(), p.Address(), err)
				// removePeer is handled by sendMessage on error
			}
		}(peer, msg) // Pass peer and message to goroutine
	}
}

// readMessageData handles reading length-prefixed messages from a net.Conn.
// This is a private helper function for readLoop.
func (s *Server) readMessageData(conn net.Conn) ([]byte, error) {
    reader := bufio.NewReader(conn)
    // Read message length (4 bytes)
    lenBuf := make([]byte, 4)
    _, err := io.ReadFull(reader, lenBuf)
    if err != nil {
        if err == io.EOF || err == io.ErrUnexpectedEOF {
            return nil, io.EOF // Propagate EOF for peer disconnection
        }
        return nil, fmt.Errorf("%w: failed to read message length: %v", ErrReceiveMessageFailed, err)
    }
    msgLen := binary.BigEndian.Uint32(lenBuf)

    // Read message data
    data := make([]byte, msgLen)
    _, err = io.ReadFull(reader, data)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to read message data: %v", ErrReceiveMessageFailed, err)
    }
    return data, nil
}

// readLoop continuously reads and processes messages from a connected peer.
func (s *Server) readLoop(p *Peer) {
	defer s.wg.Done()
	defer func() {
		s.logger.Printf("P2P: Read loop for %s exiting. Disconnecting peer.", p.Address())
		s.removePeer(p) // Ensure peer is removed on read loop exit
	}()

	s.logger.Debugf("P2P: Read loop started for %s", p.Address())

	for {
		select {
		case <-s.quit:
			return
		default:
			msgData, err := s.readMessageData(p.Conn())
			if err != nil {
				if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
					s.logger.Printf("P2P: Peer %s disconnected (EOF or connection closed).\n", p.Address())
				} else {
					s.logger.Errorf("P2P: Error reading message from peer %s: %v\n", p.Address(), err)
				}
				return // Exit read loop on error
			}

			msg, err := DeserializeMessage(msgData)
			if err != nil {
				s.logger.Errorf("P2P: Error deserializing message from peer %s: %v. Skipping message.", p.Address(), err)
				// Don't disconnect for a single malformed message; just log and continue
				continue 
			}
			
			// Update peer's last activity - crucial for liveness checks and "Sense the Landscape"
			p.UpdateLastActivity() 

			s.logger.Debugf("P2P: Received %s from %s (Length: %d)", msg.Type.String(), p.Address(), len(msgData))

			// Handle handshake messages internally first
			if msg.Type == MsgHello {
				// This branch should primarily be hit by the *initiator* receiving the *reply* hello.
				// Or if a HELLO is received from an already handshaked peer (e.g., re-handshake)
				// The initial HELLO from responder is handled by handleConnection.
				var helloPayload HelloPayload
				if err := DecodePayload(msg.Payload, &helloPayload); err != nil {
					s.logger.Errorf("P2P: Error decoding HELLO payload from %s: %v. Disconnecting.", p.Address(), err)
					return // Malformed HELLO is critical, disconnect
				}
				// Log and process known peers from their HELLO
				s.logger.Debugf("P2P: Received HELLO reply from %s (ID: %x, Height: %d, KnownPeers: %d)", p.Address(), helloPayload.NodeID, helloPayload.CurrentHeight, len(helloPayload.KnownPeers))
				
				// Update our understanding of this peer's ID (if not already set correctly during initial setup)
				// This makes peer.ID consistent with the ID advertised in HelloPayload.
				if !bytes.Equal(p.ID(), helloPayload.NodeID) {
					s.logger.Warnf("P2P: Peer %s's ID changed from %x to %x during HELLO. Updating.", p.Address(), p.ID(), helloPayload.NodeID)
					// This would require updating the map key in s.peers if we keyed by ID instead of address
					// For now, if keyed by address, we just update peer.id.
				}
				p.id = helloPayload.NodeID // Ensure peer object stores its actual ID

				// Add known peers from their HELLO to our server's global discovery list (conceptual)
				if helloPayload.ListenAddr != "" && helloPayload.ListenAddr != s.ListenAddress() { // Don't try to connect to self
					s.tryConnectToPeer(helloPayload.ListenAddr, helloPayload.NodeID) // Try to connect to new peers
				}
				for _, addr := range helloPayload.KnownPeers {
					if addr != s.ListenAddress() { // Don't try to connect to self
						s.tryConnectToPeer(addr, nil) // NodeID might not be known yet for indirect peers
					}
				}
			} else {
				// Ensure the peer is part of our active server.peers map (i.e., handshaked)
				// This guards against messages from unrecognized or already-removed peers.
				s.mu.RLock()
				_, exists := s.peers[p.Address()]
				s.mu.RUnlock()
				if !exists {
					s.logger.Warnf("P2P: Received message %s from unknown/disconnected peer %s. Ignoring.", msg.Type.String(), p.Address())
					return // Disconnect or ignore message from unknown peer
				}
				// Pass to generic message handler provided by the application.
				if s.OnMessage != nil {
					s.OnMessage(p, msg)
				} else {
					s.logger.Debugf("P2P: No handler configured for message type %s.", msg.Type.String())
				}
			}
		}
	}
}

// Connect attempts to establish an outbound connection to a remote peer.
// It performs the handshake and adds the peer to the server's list upon success.
func (s *Server) Connect(address string, remoteNodeID []byte) (*Peer, error) {
	s.mu.RLock()
	// Check if already connected by address (maintains map integrity)
	if _, exists := s.peers[address]; exists {
		s.mu.RUnlock()
		s.logger.Debugf("P2P: Already connected to peer %s", address)
		return nil, fmt.Errorf("already connected to peer %s", address) // Return error as per signature
	}
	s.mu.RUnlock()

	s.logger.Printf("P2P: Attempting to connect to %s (Remote ID: %x)", address, remoteNodeID)

	conn, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to dial peer %s: %v", ErrConnectTimeout, address, err)
	}

	peer, err := NewPeer(conn, true, remoteNodeID) // `true` as we initiated, pass remoteNodeID for conceptual consistency
	if err != nil {
		s.logger.Errorf("Failed to create peer object for %s: %v", address, err)
		conn.Close()
		return nil, err
	}

	// The handleConnection function will perform the handshake and start the read loop.
	// This function itself does not block waiting for the full handshake.
	s.wg.Add(1) // Goroutine for handleConnection
	go s.handleConnection(peer.Conn(), true) // Pass the newly created peer.Conn()

	// Note: This function returns the peer immediately, but its full handshake
	// and addition to s.peers happens asynchronously in handleConnection.
	// Callers need to be aware of this asynchronous nature.
	return peer, nil // Return the peer object created here
}


// removePeer removes a peer from the server's internal map and closes its connection.
// It also calls the OnPeerDisconnected callback.
func (s *Server) removePeer(p *Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.peers[p.Address()]; exists {
		delete(s.peers, p.Address())
		s.logger.Printf("P2P: Peer %s (ID: %x) removed.", p.Address(), p.ID())
		if s.OnPeerDisconnected != nil {
			s.OnPeerDisconnected(p)
		}
		if err := p.Close(); err != nil { // Ensure underlying connection is closed
			s.logger.Errorf("Error closing peer %s connection on removal: %v", p.Address(), err)
		}
	} else {
		s.logger.Debugf("P2P: Attempted to remove non-existent peer %s (already removed?).", p.Address())
	}
}

// addPeer adds a handshaked peer to the server's internal peer map.
// This is typically called *after* a successful handshake is complete.
func (s *Server) addPeer(p *Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Double-check if already added (e.g., if re-handshake or concurrent add)
	if existing, exists := s.peers[p.Address()]; exists {
		if !bytes.Equal(existing.ID(), p.ID()) {
			s.logger.Warnf("P2P: Peer %s already in map with different ID %x, new ID %x. Overwriting.", p.Address(), existing.ID(), p.ID())
		}
		// If same peer, ensure activity is updated
		existing.UpdateLastActivity()
		return
	}

	s.peers[p.Address()] = p
	s.logger.Printf("P2P: Peer %s (ID: %x) added to active connections. Total active: %d", p.Address(), p.ID(), len(s.peers))
	if s.OnPeerConnected != nil {
		s.OnPeerConnected(p)
	}
	// Start read loop for this new peer (if not already started by handleConnection itself)
	// No, handleConnection starts the read loop, so this is just adding to map.
}

// KnownPeers returns a list of currently connected peer addresses known by this server.
func (s *Server) KnownPeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peers := make([]string, 0, len(s.peers))
	for addr := range s.peers {
		peers = append(peers, addr)
	}
	return peers
}

// GetPeerByID returns a peer by its cryptographic ID.
// Useful for targeting specific peers.
func (s *Server) GetPeerByID(peerID []byte) *Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.peers {
		if bytes.Equal(p.ID(), peerID) {
			return p
		}
	}
	return nil
}

// GetPeerByAddress returns a peer by its network address.
func (s *Server) GetPeerByAddress(address string) *Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peers[address]
}


// tryConnectToPeer is a conceptual function to initiate outbound connections
// for newly discovered peers. This would typically be managed by a separate
// PeerDiscoveryService that feeds addresses to the Server.Connect method.
func (s *Server) tryConnectToPeer(address string, remoteNodeID []byte) {
	// This is a simplified, non-blocking attempt to connect.
	// In a real system, this would involve a connection manager,
	// backoff logic, and connection limits.
	go func() {
		if s.GetPeerByAddress(address) != nil {
			s.logger.Debugf("NETWORK: Already connected or connecting to %s, skipping tryConnectToPeer", address)
			return
		}
		s.logger.Debugf("NETWORK: Attempting background connection to discovered peer %s (ID: %x)", address, remoteNodeID)
		_, err := s.Connect(address, remoteNodeID) // Use s.Connect (which performs handshake)
		if err != nil {
			s.logger.Debugf("NETWORK: Failed background connection to %s: %v", address, err)
		}
	}()
}