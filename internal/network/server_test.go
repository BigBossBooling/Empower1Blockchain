package network

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/empower1/blockchain/internal/consensus"
	"github.com/empower1/blockchain/internal/core"
	pb "github.com/empower1/blockchain/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestServer_BroadcastAndReceive(t *testing.T) {
	// 1. Setup shared components
	pos := consensus.NewPOS()
	mpA := core.NewMempool()
	mpB := core.NewMempool()

	// 2. Setup two servers representing two nodes, sharing the same validator set
	bcA := core.NewBlockchain(pos)
	serverA := NewServer(":3000", bcA, mpA)

	bcB := core.NewBlockchain(pos)
	serverB := NewServer(":3001", bcB, mpB)

	// 3. Create an in-memory connection between them using net.Pipe
	connA, connB := net.Pipe()

	// 4. Start handlers for the connection on both servers
	go serverA.handleConn(connA)
	go serverB.handleConn(connB)

	// Add the peer to serverA so it knows who to broadcast to
	serverA.peerLock.Lock()
	serverA.peers[connB.LocalAddr()] = connA // Use the local address of the other pipe end
	serverA.peerLock.Unlock()

	// 5. Create a new block to broadcast
	genesisBlock, err := bcA.GetBlockByHeight(0)
	assert.NoError(t, err)

	proposer := pos.NextProposer()
	header := &pb.BlockHeader{
		Version:         1,
		PrevBlockHash:   genesisBlock.Block.Hash,
		Height:          1,
		Timestamp:       timestamppb.New(time.Now()),
		ProposerAddress: proposer.Address,
	}
	newBlock := core.NewBlock(header, []*pb.Transaction{})
	err = newBlock.Sign(proposer.PrivateKey())
	assert.NoError(t, err)
	newBlock.SetHash()

	// 6. Create and serialize the broadcast message
	msg := &pb.Message{
		Payload: &pb.Message_Block{
			Block: &pb.BlockMessage{Block: newBlock.Block},
		},
	}
	msgBytes, err := proto.Marshal(msg)
	assert.NoError(t, err)

	// 7. Server A broadcasts the message
	err = serverA.Broadcast(msgBytes)
	assert.NoError(t, err)

	// 8. Verification: Check if Server B received and processed the block
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, uint64(1), bcB.Height())
	receivedBlock, err := bcB.GetBlockByHeight(1)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(newBlock.Block.Hash, receivedBlock.Block.Hash))
}
