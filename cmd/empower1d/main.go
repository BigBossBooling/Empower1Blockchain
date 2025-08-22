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
	server := network.NewServer(":3000")
	go func() {
		if err := server.Start(); err != nil {
			panic(err)
		}
	}()
	fmt.Println("-> P2P Server starting...")

	// 6. Simulate the core block creation loop
	fmt.Println("--> Entering block proposal simulation loop...")
	for {
		time.Sleep(5 * time.Second) // Wait for "block time"
		proposer := pos.NextProposer()
		fmt.Printf("  [Height: %d] Next proposer: %s\n", bc.Height(), proposer)
	}
}
