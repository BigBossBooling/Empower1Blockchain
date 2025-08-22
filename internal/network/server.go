package network

import (
	"fmt"
	"net"
)

// Server represents the P2P network server for a blockchain node.
type Server struct {
	listenAddr string
}

// NewServer creates a new P2P server instance.
func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
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
			// In a real server, we might want more robust error handling
			fmt.Printf("Error accepting connection: %s\n", err)
			continue
		}

		go s.handleConn(conn)
	}
}

// handleConn manages a single incoming peer connection.
func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	peerAddr := conn.RemoteAddr().String()
	fmt.Printf("New peer connected: %s\n", peerAddr)

	// In the future, this is where we would handle the P2P message exchange loop.
	// For now, we just print a message and close the connection.
}
