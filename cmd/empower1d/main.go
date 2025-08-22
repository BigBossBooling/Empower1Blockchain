package main

import (
	"context"
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

const oracleServerAddr = "localhost:4000"

func main() {
	fmt.Println("Starting EmPower1 Node...")

	// 1. Initialize the Blockchain
	bc := core.NewBlockchain()
	fmt.Printf("-> Blockchain initialized. Current height: %d\n", bc.Height())

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
	server := network.NewServer(":3000", bc)
	go func() {
		if err := server.Start(); err != nil {
			panic(err)
		}
	}()
	fmt.Println("-> P2P Server starting...")

	// 6. Simulate the core block creation and broadcasting loop
	fmt.Println("--> Entering block creation & broadcast simulation loop...")
	for {
		time.Sleep(5 * time.Second) // Wait for "block time"

		proposer := pos.NextProposer()
		fmt.Printf("  [Height: %d] Next proposer: %s. Simulating block creation...\n", bc.Height(), proposer)

		// Get the current head of the chain to link the new block
		currentBlock, err := bc.GetBlockByHeight(bc.Height())
		if err != nil {
			log.Printf("Error getting current block: %v", err)
			continue
		}

		// Create a new block
		header := &pb.BlockHeader{
			Version:       1,
			PrevBlockHash: currentBlock.Block.Hash,
			Height:        currentBlock.Header.Height + 1,
			Timestamp:     timestamppb.New(time.Now()),
		}
		newBlock := core.NewBlock(header, []*pb.Transaction{})
		newBlock.SetHash()

		// Add the block to our own chain first
		if err := bc.AddBlock(newBlock); err != nil {
			log.Printf("Error adding new block locally: %v", err)
			continue
		}

		// Broadcast the new block to the network
		go func(b *core.Block) {
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

			fmt.Printf("Broadcasting new block (Height: %d)\n", b.Header.Height)
			if err := server.Broadcast(msgBytes); err != nil {
				log.Printf("Error broadcasting block: %v", err)
			}
		}(newBlock)
	}
}
