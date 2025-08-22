package main

import (
	"fmt"
	"time"

	"github.com/empower1/blockchain/internal/consensus"
	"github.com/empower1/blockchain/internal/core"
	"github.com/empower1/blockchain/internal/network"
)

func main() {
	fmt.Println("Starting EmPower1 Node...")

	// 1. Initialize the Blockchain
	bc := core.NewBlockchain()
	fmt.Printf("-> Blockchain initialized. Current height: %d\n", bc.Height())

	// 2. Initialize the Consensus Engine
	pos := consensus.NewPOS()
	fmt.Println("-> PoS consensus engine initialized.")

	// 3. Start the P2P Server in a separate goroutine
	server := network.NewServer(":3000")
	go func() {
		if err := server.Start(); err != nil {
			// In a real app, handle this error more gracefully
			panic(err)
		}
	}()
	fmt.Println("-> P2P Server starting...")

	// 4. Simulate the core block creation loop
	fmt.Println("--> Entering block proposal simulation loop...")
	for {
		time.Sleep(5 * time.Second) // Wait for "block time"

		proposer := pos.NextProposer()
		fmt.Printf("  [Height: %d] Next proposer: %s\n", bc.Height(), proposer)

		// In a real implementation:
		// 1. The proposer would create a new block with transactions.
		// 2. The block would be broadcast to the network.
		// 3. Other nodes would validate and add the block.
		// For now, we just simulate the proposer selection.
	}
}
