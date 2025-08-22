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
	mempool    *core.Mempool
	peerLock   sync.RWMutex
	peers      map[net.Addr]net.Conn
}

// NewServer creates a new P2P server instance.
func NewServer(listenAddr string, bc *core.Blockchain, mempool *core.Mempool) *Server {
	return &Server{
		listenAddr: listenAddr,
		bc:         bc,
		mempool:    mempool,
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

	for addr, conn := range s.peers {
		if err := s.send(conn, msg); err != nil {
			fmt.Printf("Error sending message to peer %s: %v. Dropping connection.\n", addr, err)
		}
	}
	return nil
}

// send is a helper function to send a length-prefixed message to a connection.
func (s *Server) send(conn net.Conn, msg []byte) error {
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(msg)))
	_, err := conn.Write(append(lenBuf, msg...))
	return err
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

		if err := s.handleMessage(conn, msg); err != nil {
			fmt.Printf("Error handling message from %s: %v\n", peerAddr, err)
		}
	}
}

// handleMessage dispatches incoming messages to the correct handler.
func (s *Server) handleMessage(conn net.Conn, msg *pb.Message) error {
	switch v := msg.Payload.(type) {
	case *pb.Message_Transaction:
		tx := v.Transaction.GetTransaction()
		added, err := s.mempool.Add(tx)
		if err != nil {
			return err
		}
		if added {
			fmt.Printf("Added new transaction to mempool from peer %s\n", conn.RemoteAddr())
		}
		return nil

	case *pb.Message_GetStatus:
		fmt.Printf("Received GetStatus request from %s\n", conn.RemoteAddr())
		statusMsg := &pb.Message{
			Payload: &pb.Message_Status_{
				Status_: &pb.StatusMessage{
					Height: s.bc.Height(),
				},
			},
		}
		respBytes, err := proto.Marshal(statusMsg)
		if err != nil {
			return err
		}
		return s.send(conn, respBytes)

	case *pb.Message_GetBlocks:
		req := v.GetBlocks
		fmt.Printf("Received GetBlocks request from %s for range %d-%d\n", conn.RemoteAddr(), req.FromHeight, req.ToHeight)
		for i := req.FromHeight; i <= req.ToHeight; i++ {
			block, err := s.bc.GetBlockByHeight(i)
			if err != nil {
				return err
			}
			blockMsg := &pb.Message{
				Payload: &pb.Message_Block{
					Block: &pb.BlockMessage{Block: block.Block},
				},
			}
			respBytes, err := proto.Marshal(blockMsg)
			if err != nil {
				return err
			}
			if err := s.send(conn, respBytes); err != nil {
				return fmt.Errorf("error sending block %d: %v", i, err)
			}
		}
		return nil

	case *pb.Message_Block:
		block := v.Block.GetBlock()
		fmt.Printf("Received new block message from %s. Height: %d\n", conn.RemoteAddr(), block.Header.Height)
		return s.bc.AddBlock(&core.Block{Block: block})

	default:
		return fmt.Errorf("received unknown message type: %T", v)
	}
}
