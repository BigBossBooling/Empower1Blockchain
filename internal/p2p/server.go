package p2p

import (
	"bufio"
	"bytes"         // Added bytes import
	"encoding/binary" // Added encoding/binary import
	"encoding/gob"  // Added encoding/gob import
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	protocolVersion = "empower1/0.1"
	messageHeaderLen = 1 // For message type
)

// Server manages incoming peer connections and message broadcasting.
type Server struct {
	listenAddress string
	listener      net.Listener
	peers         map[string]*Peer
	mu            sync.RWMutex
	quit          chan struct{}
	wg            sync.WaitGroup

	// OnPeerConnected is called when a new peer successfully handshakes.
	OnPeerConnected func(p *Peer)
	// OnPeerDisconnected is called when a peer disconnects.
	OnPeerDisconnected func(p *Peer)
	// OnMessage is called when a message is received from a peer.
	// It's the responsibility of the handler to process the message.
	OnMessage func(p *Peer, msg *Message)
}

// Mu returns the RWMutex for the server, allowing external access for locking if necessary.
// Use with caution, prefer methods on Server that handle locking internally.
func (s *Server) Mu() *sync.RWMutex {
	return &s.mu
}

// ListenAddress returns the address the server is listening on.
func (s *Server) ListenAddress() string {
	if s.listener == nil {
		return s.listenAddress // Return configured address if listener not active yet
	}
	return s.listener.Addr().String() // Return actual bound address
}

// QuitSignal returns the quit channel, which is closed when the server is stopping.
func (s *Server) QuitSignal() <-chan struct{} {
	return s.quit
}

// NewServer creates a new P2P server.
func NewServer(listenAddress string) *Server {
	return &Server{
		listenAddress: listenAddress,
		peers:         make(map[string]*Peer),
		quit:          make(chan struct{}),
	}
}

// Start initializes and starts the P2P server.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.listenAddress, err)
	}
	s.listener = ln
	log.Printf("P2P server listening on %s\n", s.listenAddress)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop gracefully shuts down the P2P server.
func (s *Server) Stop() {
	close(s.quit)
	if s.listener != nil {
		s.listener.Close()
	}

	s.mu.Lock()
	for _, peer := range s.peers {
		peer.Close()
	}
	s.peers = make(map[string]*Peer)
	s.mu.Unlock()

	s.wg.Wait()
	log.Println("P2P server stopped.")
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.quit:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				// Check if the error is due to listener being closed
				select {
				case <-s.quit:
					return
				default:
					log.Printf("Failed to accept connection: %v\n", err)
					// Avoid busy-looping on accept errors
					if ne, ok := err.(net.Error); ok && ne.Temporary() {
						time.Sleep(100 * time.Millisecond)
					}
				}
				continue
			}
			go s.handleNewConnection(conn, false) // false because this peer initiated connection to us
		}
	}
}

func (s *Server) handleNewConnection(conn net.Conn, initiator bool) {
	peerAddr := conn.RemoteAddr().String()
	log.Printf("New connection from %s (initiator: %t)\n", peerAddr, initiator)

	peer := NewPeer(conn, initiator)

	// Perform handshake
	if initiator { // We initiated connection
		err := s.sendHello(peer)
		if err != nil {
			log.Printf("Failed to send HELLO to %s: %v", peerAddr, err)
			conn.Close()
			return
		}
	}

	// Read loop for the new peer
	s.wg.Add(1)
	go s.readLoop(peer)
}

func (s *Server) addPeer(peer *Peer) {
	s.mu.Lock()
	s.peers[peer.Address()] = peer
	s.mu.Unlock()
	log.Printf("Peer %s added.\n", peer.Address())
	if s.OnPeerConnected != nil {
		s.OnPeerConnected(peer)
	}
}

func (s *Server) removePeer(peer *Peer) {
	s.mu.Lock()
	delete(s.peers, peer.Address())
	s.mu.Unlock()
	log.Printf("Peer %s removed.\n", peer.Address())
	if s.OnPeerDisconnected != nil {
		s.OnPeerDisconnected(peer)
	}
	peer.Close()
}

// KnownPeers returns a list of currently connected peer addresses.
func (s *Server) KnownPeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peers := make([]string, 0, len(s.peers))
	for addr := range s.peers {
		peers = append(peers, addr)
	}
	return peers
}

func (s *Server) sendHello(p *Peer) error {
	helloPayload := HelloPayload{
		Version:    protocolVersion,
		ListenAddr: s.listenAddress, // Share our listening address
		KnownPeers: s.KnownPeers(),  // Share our currently connected peers
	}
	payloadBytes, err := ToBytes(helloPayload) // Use generic ToBytes
	if err != nil {
		return fmt.Errorf("failed to serialize HELLO payload: %w", err)
	}
	msg := Message{Type: MsgHello, Payload: payloadBytes}
	return s.SendMessage(p, &msg)
}

// SendPeerList sends a list of peer addresses to the specified peer.
func (s *Server) SendPeerList(p *Peer, peersToSend []string) error {
	peerListPayload := PeerListPayload{Peers: peersToSend}
	payloadBytes, err := ToBytes(peerListPayload) // Use generic ToBytes
	if err != nil {
		return fmt.Errorf("failed to serialize PEER_LIST payload: %w", err)
	}
	msg := Message{Type: MsgPeerList, Payload: payloadBytes}
	return s.SendMessage(p, &msg)
}

func (s *Server) sendRequestPeerList(p *Peer) error {
	msg := Message{Type: MsgRequestPeerList, Payload: []byte{}}
	return s.SendMessage(p, &msg)
}

// SendMessage sends a message to a specific peer.
func (s *Server) SendMessage(p *Peer, msg *Message) error {
	data, err := msg.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize message for peer %s: %w", p.Address(), err)
	}

	// Simple framing: send length of message, then message
	// For robustness, a more complex framing might be needed (e.g., magic bytes, checksums)
	msgLen := uint32(len(data))
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, msgLen)

	_, err = p.Conn().Write(lenBuf)
	if err != nil {
		s.removePeer(p)
		return fmt.Errorf("failed to send message length to peer %s: %w", p.Address(), err)
	}

	_, err = p.Conn().Write(data)
	if err != nil {
		s.removePeer(p)
		return fmt.Errorf("failed to send message data to peer %s: %w", p.Address(), err)
	}
	// log.Printf("Sent %s to %s", msg.Type.String(), p.Address())
	return nil
}

// BroadcastMessage sends a message to all connected peers except the sender (if specified).
func (s *Server) BroadcastMessage(msg *Message, excludePeer *Peer) {
	s.mu.RLock()
	peersToBroadcast := make([]*Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		if excludePeer != nil && peer.Address() == excludePeer.Address() {
			continue
		}
		peersToBroadcast = append(peersToBroadcast, peer)
	}
	s.mu.RUnlock() // Unlock before sending to avoid deadlock if SendMessage calls removePeer

	for _, peer := range peersToBroadcast {
		if err := s.SendMessage(peer, msg); err != nil {
			log.Printf("Error broadcasting message to peer %s: %v\n", peer.Address(), err)
			// removePeer is handled by SendMessage on error
		}
	}
}

func (s *Server) readLoop(p *Peer) {
	defer s.wg.Done()
	defer func() {
		log.Printf("Closing connection and removing peer %s", p.Address())
		s.removePeer(p)
	}()

	reader := bufio.NewReader(p.Conn())

	for {
		select {
		case <-s.quit:
			return
		default:
			// Read message length (4 bytes)
			lenBuf := make([]byte, 4)
			_, err := io.ReadFull(reader, lenBuf)
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF || strings.Contains(err.Error(), "use of closed network connection") {
					log.Printf("Peer %s disconnected (EOF or closed).\n", p.Address())
				} else {
					log.Printf("Error reading message length from peer %s: %v\n", p.Address(), err)
				}
				return
			}
			msgLen := binary.BigEndian.Uint32(lenBuf)

			// Read message data
			data := make([]byte, msgLen)
			_, err = io.ReadFull(reader, data)
			if err != nil {
				log.Printf("Error reading message data from peer %s: %v\n", p.Address(), err)
				return
			}

			msg, err := DeserializeMessage(data)
			if err != nil {
				log.Printf("Error deserializing message from peer %s: %v\n", p.Address(), err)
				continue // Don't disconnect for a bad message, just log and continue
			}

			// log.Printf("Received %s from %s", msg.Type.String(), p.Address())

			// Handle handshake internally first
			if msg.Type == MsgHello {
				var helloPayload HelloPayload
				if err := gob.NewDecoder(bytes.NewReader(msg.Payload)).Decode(&helloPayload); err != nil {
					log.Printf("Error deserializing HELLO payload from %s: %v", p.Address(), err)
					return // Disconnect if HELLO is malformed
				}

				// Basic validation
				if !strings.HasPrefix(helloPayload.Version, "empower1/") {
					log.Printf("Peer %s has incompatible protocol version: %s", p.Address(), helloPayload.Version)
					return // Disconnect
				}
				log.Printf("Received HELLO from %s: version=%s, listenAddr=%s, knownPeers=%d", p.Address(), helloPayload.Version, helloPayload.ListenAddr, len(helloPayload.KnownPeers))

				// Add peer to server's list *after* successful handshake
				s.addPeer(p)

				// If the peer is also a listener, store its listen address
				if helloPayload.ListenAddr != "" {
					p.AddKnownPeer(helloPayload.ListenAddr) // The peer itself is a known peer
				}
				// Add peers from their hello message
				for _, addr := range helloPayload.KnownPeers {
					p.AddKnownPeer(addr) // Store peers known by this peer
					// Potentially try to connect to these new peers (see peer discovery logic)
				}

				// If we didn't initiate, send our HELLO back (ack) and our peer list.
				if !p.isInitiator {
					if err := s.sendHello(p); err != nil {
						log.Printf("Failed to send reply HELLO to %s: %v", p.Address(), err)
						return // Disconnect
					}
				}
				// Send our current peer list to the new peer
				// This helps in peer discovery.
				// Avoid sending an empty list if we just started and only know them.
				ourKnownPeers := s.KnownPeers()
				if len(ourKnownPeers) > 1 || (len(ourKnownPeers) == 1 && ourKnownPeers[0] != p.Address()) {
					s.SendPeerList(p, ourKnownPeers) // Changed to exported method
				}


				// Request their peer list if they didn't send one or to refresh
				// (optional, could be periodic)
				// s.sendRequestPeerList(p)

			} else {
				// For other messages, ensure the peer is already added (handshaked)
				s.mu.RLock()
				_, exists := s.peers[p.Address()]
				s.mu.RUnlock()
				if !exists {
					log.Printf("Received message from non-handshaked peer %s. Disconnecting.", p.Address())
					return
				}
				// Pass to generic message handler
				if s.OnMessage != nil {
					s.OnMessage(p, msg)
				}
			}
		}
	}
}

// Connect attempts to connect to a remote peer and adds it to the server's peer list.
// This is a simplified client function integrated into the server for outgoing connections.
func (s *Server) Connect(address string) (*Peer, error) {
	s.mu.RLock()
	if _, exists := s.peers[address]; exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("already connected to peer %s", address)
	}
	s.mu.RUnlock()

	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer %s: %w", address, err)
	}

	log.Printf("Attempting to connect to %s\n", address)
	// The handleNewConnection function will take care of the rest, including handshake
	// true because we initiated this connection
	go s.handleNewConnection(conn, true)

	// Note: The peer is added to s.peers asynchronously by handleNewConnection after handshake.
	// This function might return before the peer is fully handshaked and added.
	// For simplicity, we return nil for the peer here. A more robust implementation
	// might use a channel or callback to signal successful connection and return the Peer object.
	return nil, nil // Or return the conn, but peer object is better.
}
