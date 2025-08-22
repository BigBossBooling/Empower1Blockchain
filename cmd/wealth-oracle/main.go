package main

import (
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const listenAddr = ":4000"

// oracleServer implements the OracleServiceServer interface generated from our protobuf.
type oracleServer struct {
	pb.UnimplementedOracleServiceServer
}

// GetWealthScores is the implementation of the gRPC service method.
// It streams a hardcoded list of wealth scores to the client.
func (s *oracleServer) GetWealthScores(req *pb.GetWealthScoresRequest, stream pb.OracleService_GetWealthScoresServer) error {
	log.Println("Client connected, streaming wealth scores...")

	// In a real implementation, this data would be calculated from on-chain/off-chain sources.
	// For now, we use a hardcoded list.
	records := []*pb.WealthScoreRecord{
		{UserId: "user-alice", Score: 78.5, UpdatedAt: timestamppb.New(time.Now())},
		{UserId: "user-bob", Score: 23.2, UpdatedAt: timestamppb.New(time.Now())},
		{UserId: "user-charlie", Score: 95.1, UpdatedAt: timestamppb.New(time.Now())},
	}

	for _, record := range records {
		if err := stream.Send(record); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond) // Simulate some processing delay
	}

	log.Println("Finished streaming scores.")
	return nil
}

func main() {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddr, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterOracleServiceServer(grpcServer, &oracleServer{})

	fmt.Printf("Wealth Oracle gRPC server listening on %s\n", listenAddr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}
}
