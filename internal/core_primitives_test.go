package internal

import (
	"testing"
	"time"

	pb "empower1/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestBlockMarshalling(t *testing.T) {
	// 1. Instantiate: Create a sample Block.
	originalBlock := &pb.Block{
		Header: &pb.BlockHeader{
			PrevBlockHash: []byte("previous_hash_placeholder"),
			MerkleRoot:    []byte("merkle_root_placeholder"),
			Timestamp:     time.Now().Unix(),
			Nonce:         1,
		},
		Transactions: [][]byte{
			[]byte("tx1"),
			[]byte("tx2"),
		},
	}
	t.Logf("Original Block: %+v", originalBlock)

	// 2. Marshal: Convert the struct into its binary wire format.
	data, err := proto.Marshal(originalBlock)
	assert.NoError(t, err, "Marshalling should not produce an error")
	assert.NotEmpty(t, data, "Marshalled data should not be empty")
	t.Logf("Successfully marshalled block to %d bytes", len(data))

	// 3. Unmarshal: Convert the bytes back into a new struct.
	newBlock := &pb.Block{}
	err = proto.Unmarshal(data, newBlock)
	assert.NoError(t, err, "Unmarshalling should not produce an error")
	t.Logf("Successfully unmarshalled data back to block struct: %+v", newBlock)

	// 4. Assert: Verify the unmarshaled struct is a perfect match.
	assert.True(t, proto.Equal(originalBlock, newBlock), "Original and unmarshalled blocks should be identical")
	t.Log("SUCCESS: Verified that the Block primitive maintains perfect integrity after a marshal-unmarshal loop.")
}

func TestTransactionMarshalling(t *testing.T) {
	// 1. Instantiate: Create a sample Transaction.
	originalTx := &pb.Transaction{
		From:      []byte("from_address"),
		To:        []byte("to_address"),
		Amount:    100,
		Fee:       1,
		Signature: []byte("signature"),
	}
	t.Logf("Original Transaction: %+v", originalTx)

	// 2. Marshal
	data, err := proto.Marshal(originalTx)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	t.Logf("Successfully marshalled transaction to %d bytes", len(data))

	// 3. Unmarshal
	newTx := &pb.Transaction{}
	err = proto.Unmarshal(data, newTx)
	assert.NoError(t, err)
	t.Logf("Successfully unmarshalled data back to transaction struct: %+v", newTx)

	// 4. Assert
	assert.True(t, proto.Equal(originalTx, newTx), "Original and unmarshalled transactions should be identical")
	t.Log("SUCCESS: Verified that the Transaction primitive maintains perfect integrity after a marshal-unmarshal loop.")
}
