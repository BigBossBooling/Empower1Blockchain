package engine

import (
	"context"
	"io"
	"log"

	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OracleClient is a client for the Wealth Oracle gRPC service.
type OracleClient struct {
	client pb.OracleServiceClient
}

// NewOracleClient creates a new client and connects to the oracle server.
func NewOracleClient(oracleAddr string) (*OracleClient, error) {
	conn, err := grpc.Dial(oracleAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewOracleServiceClient(conn)
	return &OracleClient{
		client: client,
	}, nil
}

// FetchWealthScores connects to the oracle and fetches the latest wealth scores.
func (c *OracleClient) FetchWealthScores(ctx context.Context) ([]*pb.WealthScoreRecord, error) {
	stream, err := c.client.GetWealthScores(ctx, &pb.GetWealthScoresRequest{})
	if err != nil {
		return nil, err
	}

	var records []*pb.WealthScoreRecord
	for {
		record, err := stream.Recv()
		if err == io.EOF {
			break // End of stream
		}
		if err != nil {
			log.Printf("Error receiving from oracle stream: %v", err)
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}
