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
	"github.com/empower1/blockchain/internal/crypto"
	"github.com/empower1/blockchain/internal/engine"
	"github.com/empower1/blockchain/internal/network"
	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = crypto.NewPrivateKey // Acknowledge crypto package usage

const (
	p2pListenAddr    = ":3000"
	oracleServerAddr = "localhost:4000"
)

var bootstrapNodes = []string{":3000"}

func main() {
	fmt.Println("Starting EmPower1 Node...")

	// 1. Initialize the Consensus Engine
	pos := consensus.NewPOS()
	fmt.Println("-> PoS consensus engine initialized.")

	// 2. Initialize core components, passing in the consensus engine
	bc := core.NewBlockchain(pos)
	mempool := core.NewMempool()
	fmt.Printf("-> Blockchain initialized. Current height: %d\n", bc.Height())
	fmt.Println("-> Mempool initialized.")

	// 3. Initialize the Redistribution Engine and Oracle Client
	redistributionEngine := engine.New()
	oracleClient, err := engine.NewOracleClient(oracleServerAddr)
	if err != nil {
		log.Fatalf("Failed to connect to oracle server: %v", err)
	}
	fmt.Println("-> Redistribution Engine and Oracle Client initialized.")

	// 4. Start background goroutines
	go startScoreFetchingLoop(redistributionEngine, oracleClient)
	server := startP2PServer(p2pListenAddr, bc, mempool)
	time.Sleep(100 * time.Millisecond) // Give the server a moment to start

	// 5. Run initial chain synchronization
	syncer := network.NewSyncer(bc, bootstrapNodes)
	if err := syncer.Start(); err != nil {
		log.Printf("Chain synchronization failed: %v", err)
	}

	// 6. Start the core block creation loop
	fmt.Println("--> Entering block creation & broadcast simulation loop...")
	blockCreationLoop(pos, bc, mempool, server)
}

func startScoreFetchingLoop(e *engine.RedistributionEngine, c *engine.OracleClient) {
	for {
		fmt.Println("--> Fetching wealth scores from oracle...")
		scores, err := c.FetchWealthScores(context.Background())
		if err != nil {
			log.Printf("Error fetching scores: %v", err)
		} else {
			e.UpdateScores(scores)
		}
		time.Sleep(1 * time.Minute)
	}
}

func startP2PServer(addr string, bc *core.Blockchain, mp *core.Mempool) *network.Server {
	server := network.NewServer(addr, bc, mp)
	go func() {
		if err := server.Start(); err != nil {
			panic(err)
		}
	}()
	fmt.Println("-> P2P Server starting...")
	return server
}

func blockCreationLoop(pos *consensus.POS, bc *core.Blockchain, mp *core.Mempool, s *network.Server) {
	for {
		time.Sleep(5 * time.Second)

		proposer := pos.NextProposer()
		fmt.Printf("  [Height: %d] Next proposer: %s. Simulating block creation...\n", bc.Height(), proposer.Address)

		pendingTxs := mp.GetPending()
		fmt.Printf("  Found %d pending transactions in mempool.\n", len(pendingTxs))

		prevBlock, err := bc.GetBlockByHeight(bc.Height())
		if err != nil {
			log.Printf("Error getting current block: %v", err)
			continue
		}

		newBlock, err := createNewBlock(prevBlock, proposer, pendingTxs)
		if err != nil {
			log.Printf("Error creating new block: %v", err)
			continue
		}

		if err := bc.AddBlock(newBlock); err != nil {
			log.Printf("Error adding new block locally: %v", err)
			continue
		}

		mp.Clear()
		go broadcastBlock(s, newBlock)
	}
}

func createNewBlock(prevBlock *core.Block, proposer *consensus.Validator, txs []*pb.Transaction) (*core.Block, error) {
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
		ProposerAddress:  proposer.Address,
	}

	block := core.NewBlock(header, txs)
	if err := block.Sign(proposer.PrivateKey()); err != nil {
		return nil, err
	}
	if err := block.SetHash(); err != nil {
		return nil, err
	}

	return block, nil
}

func calculateTxsHash(txs []*pb.Transaction) ([]byte, error) {
	if len(txs) == 0 {
		return make([]byte, 32), nil
	}
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
