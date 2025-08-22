package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"time"

	"github.com/empower1/blockchain/internal/consensus"
	"github.com/empower1/blockchain/internal/core"
	"github.com/empower1/blockchain/internal/engine"
	"github.com/empower1/blockchain/internal/network"
	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	p2pListenAddr    = ":3000"
	oracleServerAddr = "localhost:4000"
)

var bootstrapNodes = []string{":3000"} // For now, it will try to sync with itself

func main() {
	fmt.Println("Starting EmPower1 Node...")

	// 1. Initialize core components
	bc := core.NewBlockchain()
	mempool := core.NewMempool()
	fmt.Printf("-> Blockchain initialized. Current height: %d\n", bc.Height())
	fmt.Println("-> Mempool initialized.")

	// 2. Initialize the Consensus Engine
	pos := consensus.NewPOS()
	fmt.Println("-> PoS consensus engine initialized.")

	// 3. Initialize the Redistribution Engine and Oracle Client
	redistributionEngine := engine.New()
	oracleClient, err := engine.NewOracleClient(oracleServerAddr)
	if err != nil {
		log.Fatalf("Failed to connect to oracle server: %v", err)
	}
	fmt.Println("-> Redistribution Engine and Oracle Client initialized.")

	// 4. Start a background goroutine to fetch scores from the oracle
	go func() {
		for {
			fmt.Println("--> Fetching wealth scores from oracle...")
			scores, err := oracleClient.FetchWealthScores(context.Background())
			if err != nil {
				log.Printf("Error fetching scores: %v", err)
			} else {
				redistributionEngine.UpdateScores(scores)
			}
			time.Sleep(1 * time.Minute) // Fetch scores every minute
		}
	}()

	// 5. Start the P2P Server in a separate goroutine
	server := network.NewServer(p2pListenAddr, bc, mempool)
	go func() {
		if err := server.Start(); err != nil {
			panic(err)
		}
	}()
	fmt.Println("-> P2P Server starting...")
	time.Sleep(100 * time.Millisecond) // Give the server a moment to start

	// 6. Run initial chain synchronization
	syncer := network.NewSyncer(bc, bootstrapNodes)
	if err := syncer.Start(); err != nil {
		log.Printf("Chain synchronization failed: %v", err)
	}

	// 7. Simulate the core block creation and broadcasting loop
	fmt.Println("--> Entering block creation & broadcast simulation loop...")
	for {
		time.Sleep(5 * time.Second) // Wait for "block time"

		proposer := pos.NextProposer()
		fmt.Printf("  [Height: %d] Next proposer: %s. Simulating block creation...\n", bc.Height(), proposer)

		// Get pending transactions from the mempool
		pendingTxs := mempool.GetPending()
		fmt.Printf("  Found %d pending transactions in mempool.\n", len(pendingTxs))

		currentBlock, err := bc.GetBlockByHeight(bc.Height())
		if err != nil {
			log.Printf("Error getting current block: %v", err)
			continue
		}

		newBlock, err := createNewBlock(currentBlock, pendingTxs)
		if err != nil {
			log.Printf("Error creating new block: %v", err)
			continue
		}

		if err := bc.AddBlock(newBlock); err != nil {
			log.Printf("Error adding new block locally: %v", err)
			continue
		}

		// Clear the mempool now that transactions are included in a block
		mempool.Clear()

		go broadcastBlock(server, newBlock)
	}
}

func createNewBlock(prevBlock *core.Block, txs []*pb.Transaction) (*core.Block, error) {
	txsHash, err := calculateTxsHash(txs)
	if err != nil {
		return nil, err
	}

	header := &pb.BlockHeader{
		Version:          1,
		PrevBlockHash:    prevBlock.Block.Hash,
		TransactionsHash: txsHash,
		Height:           prevBlock.Header.Height + 1,
		Timestamp:        timestamppb.New(time.Now()),
	}
	newBlock := core.NewBlock(header, txs)
	newBlock.SetHash()
	return newBlock, nil
}

// calculateTxsHash is a placeholder for a proper Merkle root calculation.
func calculateTxsHash(txs []*pb.Transaction) ([]byte, error) {
	if len(txs) == 0 {
		return make([]byte, 32), nil
	}
	// Simple approach: concatenate all tx hashes and hash the result.
	var txHashes [][]byte
	for _, tx := range txs {
		txBytes, err := proto.Marshal(tx)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(txBytes)
		txHashes = append(txHashes, hash[:])
	}
	combined := bytes.Join(txHashes, []byte{})
	finalHash := sha256.Sum256(combined)
	return finalHash[:], nil
}

func broadcastBlock(s *network.Server, b *core.Block) {
	msg := &pb.Message{
		Payload: &pb.Message_Block{
			Block: &pb.BlockMessage{Block: b.Block},
		},
	}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error marshalling block message: %v", err)
		return
	}

	fmt.Printf("Broadcasting new block (Height: %d) with %d transactions\n", b.Header.Height, len(b.Transactions))
	if err := s.Broadcast(msgBytes); err != nil {
		log.Printf("Error broadcasting block: %v", err)
	}
}
