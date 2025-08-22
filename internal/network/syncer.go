package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/empower1/blockchain/internal/core"
	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/protobuf/proto"
)

// Syncer is responsible for syncing the blockchain with peers.
type Syncer struct {
	bc             *core.Blockchain
	bootstrapNodes []string
}

// NewSyncer creates a new syncer component.
func NewSyncer(bc *core.Blockchain, bootstrapNodes []string) *Syncer {
	return &Syncer{
		bc:             bc,
		bootstrapNodes: bootstrapNodes,
	}
}

// Start initiates the synchronization process.
func (s *Syncer) Start() error {
	fmt.Println("Starting chain synchronization...")
	for _, addr := range s.bootstrapNodes {
		err := s.syncWithPeer(addr)
		if err == nil {
			fmt.Println("Chain synchronization successful with peer:", addr)
			return nil // Stop after first successful sync for simplicity
		}
		fmt.Printf("Failed to sync with peer %s: %v. Trying next peer.\n", addr, err)
	}
	fmt.Println("Finished synchronization attempts.")
	return nil
}

// syncWithPeer handles the synchronization logic with a single peer.
func (s *Syncer) syncWithPeer(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Printf("Connected to peer: %s\n", addr)

	// 1. Send GetStatus message
	getStatusMsg := &pb.Message{Payload: &pb.Message_GetStatus{GetStatus: &pb.GetStatusMessage{}}}
	if err := sendMsg(conn, getStatusMsg); err != nil {
		return fmt.Errorf("failed to send get_status message: %v", err)
	}

	// 2. Read Status response
	msg, err := readMsg(conn)
	if err != nil {
		return fmt.Errorf("failed to read status response: %v", err)
	}

	statusMsg, ok := msg.Payload.(*pb.Message_Status_)
	if !ok {
		return fmt.Errorf("unexpected message type received, expected StatusMessage")
	}

	peerHeight := statusMsg.Status_.GetHeight()
	localHeight := s.bc.Height()
	fmt.Printf("Local height: %d, Peer height: %d\n", localHeight, peerHeight)

	// 3. If peer has a longer chain, request missing blocks
	if peerHeight > localHeight {
		fmt.Printf("Peer has a longer chain. Requesting blocks from height %d...\n", localHeight+1)
		getBlocksMsg := &pb.Message{
			Payload: &pb.Message_GetBlocks{
				GetBlocks: &pb.GetBlocksMessage{
					FromHeight: localHeight + 1,
					ToHeight:   peerHeight,
				},
			},
		}
		if err := sendMsg(conn, getBlocksMsg); err != nil {
			return fmt.Errorf("failed to send get_blocks message: %v", err)
		}

		// 4. Read the stream of blocks and add them to our chain
		numToSync := peerHeight - localHeight
		fmt.Printf("Need to sync %d blocks.\n", numToSync)

		for i := uint64(0); i < numToSync; i++ {
			blockMsg, err := readMsg(conn)
			if err != nil {
				return fmt.Errorf("error reading block stream on block %d: %v", i, err)
			}

			if blockPayload, ok := blockMsg.Payload.(*pb.Message_Block); ok {
				block := blockPayload.Block.GetBlock()
				if err := s.bc.AddBlock(&core.Block{Block: block}); err != nil {
					return fmt.Errorf("failed to add synced block: %v", err)
				}
				fmt.Printf("Added synced block. New height: %d\n", s.bc.Height())
			} else {
				return fmt.Errorf("unexpected message type during block sync: expected BlockMessage")
			}
		}
	} else {
		fmt.Println("Local chain is already up-to-date with this peer.")
	}

	return nil
}

// sendMsg is a helper to send a single message to a connection.
func sendMsg(conn net.Conn, msg *pb.Message) error {
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(msgBytes)))
	_, err = conn.Write(append(lenBuf, msgBytes...))
	return err
}

// readMsg is a helper to read a single message from a connection.
func readMsg(conn net.Conn) (*pb.Message, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)

	msgBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgBuf); err != nil {
		return nil, err
	}

	msg := &pb.Message{}
	if err := proto.Unmarshal(msgBuf, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
