package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const nodeAddr = "localhost:3000"

func main() {
	conn, err := net.Dial("tcp", nodeAddr)
	if err != nil {
		log.Fatalf("Failed to connect to node at %s: %v", nodeAddr, err)
	}
	defer conn.Close()

	fmt.Printf("Connected to node: %s\n", nodeAddr)

	// 1. Create a dummy transaction
	tx := &pb.Transaction{
		From:      "test-sender",
		To:        "test-receiver",
		Value:     100,
		Timestamp: timestamppb.New(time.Now()),
	}

	// 2. Wrap it in a TransactionMessage and then a top-level Message
	txMsg := &pb.TransactionMessage{Transaction: tx}
	msg := &pb.Message{
		Payload: &pb.Message_Transaction{
			Transaction: txMsg,
		},
	}

	// 3. Serialize the message
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Fatalf("Failed to marshal transaction message: %v", err)
	}

	// 4. Send the message with a length prefix
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(msgBytes)))
	if _, err := conn.Write(append(lenBuf, msgBytes...)); err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	fmt.Println("Successfully sent dummy transaction to the node.")
}
