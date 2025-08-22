package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/empower1/blockchain/internal/core"
	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/protobuf/proto"
)

// Server represents the P2P network server for a blockchain node.
type Server struct {
	listenAddr string
	bc         *core.Blockchain
	peerLock   sync.RWMutex
	peers      map[net.Addr]net.Conn
}

// NewServer creates a new P2P server instance.
func NewServer(listenAddr string, bc *core.Blockchain) *Server {
	return &Server{
		listenAddr: listenAddr,
		bc:         bc,
		peers:      make(map[net.Addr]net.Conn),
	}
}

// Start initializes the server, listens for incoming connections,
// and handles them in separate goroutines.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("P2P Server listening on %s\n", s.listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %s\n", err)
			continue
		}

		go s.handleConn(conn)
	}
}

// Broadcast sends a message to all connected peers.
func (s *Server) Broadcast(msg []byte) error {
	s.peerLock.RLock()
	defer s.peerLock.RUnlock()

	// Prepare the length-prefixed message
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(msg)))
	fullMsg := append(lenBuf, msg...)

	for addr, conn := range s.peers {
		_, err := conn.Write(fullMsg)
		if err != nil {
			fmt.Printf("Error sending message to peer %s: %v. Dropping connection.\n", addr, err)
			// In a real implementation, we would handle this more gracefully,
			// possibly removing the peer from the list.
		}
	}
	return nil
}

// handleConn manages a single incoming peer connection.
func (s *Server) handleConn(conn net.Conn) {
	peerAddr := conn.RemoteAddr()
	s.peerLock.Lock()
	s.peers[peerAddr] = conn
	s.peerLock.Unlock()

	defer func() {
		s.peerLock.Lock()
		delete(s.peers, peerAddr)
		s.peerLock.Unlock()
		conn.Close()
		fmt.Printf("Peer %s disconnected.\n", peerAddr)
	}()

	fmt.Printf("New peer connected: %s\n", peerAddr)

	// Loop to read and handle messages from the peer
	for {
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				fmt.Printf("Error reading message length from %s: %v\n", peerAddr, err)
			}
			break
		}
		msgLen := binary.BigEndian.Uint32(lenBuf)

		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, msgBuf); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				fmt.Printf("Error reading message payload from %s: %v\n", peerAddr, err)
			}
			break
		}

		msg := &pb.Message{}
		if err := proto.Unmarshal(msgBuf, msg); err != nil {
			fmt.Printf("Error decoding message from %s: %v\n", peerAddr, err)
			continue
		}

		if err := s.handleMessage(msg); err != nil {
			fmt.Printf("Error handling message from %s: %v\n", peerAddr, err)
		}
	}
}

// handleMessage dispatches incoming messages to the correct handler.
func (s *Server) handleMessage(msg *pb.Message) error {
	switch v := msg.Payload.(type) {
	case *pb.Message_Block:
		block := v.Block.GetBlock()
		fmt.Printf("Received new block message. Height: %d\n", block.Header.Height)
		return s.bc.AddBlock(&core.Block{Block: block})
	default:
		return fmt.Errorf("received unknown message type: %T", v)
	}
}
