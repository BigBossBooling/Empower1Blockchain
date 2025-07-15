package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/empower1/empower1/proto"
)

type Block struct {
	*proto.Block
}

func NewBlock(prevHash []byte, transactions []*Transaction, validator []byte) *Block {
	block := &Block{
		Block: &proto.Block{
			PrevHash:     prevHash,
			Timestamp:    uint64(time.Now().Unix()),
			Transactions: make([]*proto.Transaction, len(transactions)),
			Validator:    validator,
		},
	}

	for i, tx := range transactions {
		block.Block.Transactions[i] = tx.Transaction
	}

	block.SetHash()

	return block
}

func (b *Block) SetHash() {
	timestamp := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestamp, b.Timestamp)
	headers := bytes.Join([][]byte{b.PrevHash, b.HashTransactions(), timestamp}, []byte{})
	hash := sha256.Sum256(headers)
	b.Hash = hash[:]
}

func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Hash)
	}
	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))

	return txHash[:]
}

func (b *Block) GetHash() string {
	return hex.EncodeToString(b.Hash)
}
