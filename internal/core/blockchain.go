package core

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"log"

	"github.com/boltdb/bolt"
	"github.com/empower1/empower1/internal/consensus"
)

const dbFile = "blockchain.db"
const blocksBucket = "blocks"

type Blockchain struct {
	tip      []byte
	db       *bolt.DB
	manager *consensus.Manager
}

func (bc *Blockchain) Db() *bolt.DB {
	return bc.db
}

func NewBlockchain() *Blockchain {
	var tip []byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	manager := consensus.NewManager()
	manager.AddValidator([]byte("genesis"), 100)

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			genesis := NewGenesisBlock()
			b, err := tx.CreateBucket([]byte(blocksBucket))
			if err != nil {
				log.Panic(err)
			}
			err = b.Put(genesis.Hash, genesis.Serialize())
			if err != nil {
				log.Panic(err)
			}
			err = b.Put([]byte("l"), genesis.Hash)
			if err != nil {
				log.Panic(err)
			}
			tip = genesis.Hash
		} else {
			tip = b.Get([]byte("l"))
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	bc := Blockchain{tip, db, manager}

	return &bc
}

func (bc *Blockchain) AddBlock(transactions []*Transaction) {
	var lastHash []byte

	err := bc.Db().View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("l"))

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	validator, err := bc.manager.SelectNextValidator()
	if err != nil {
		log.Panic(err)
	}

	newBlock := NewBlock(lastHash, transactions, validator)

	err = bc.Db().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("l"), newBlock.Hash)
		if err != nil {
			log.Panic(err)
		}

		bc.tip = newBlock.Hash

		return nil
	})

	err = bc.Db().View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		bc.manager.RecordBlockProduction(validator, uint64(b.Stats().KeyN))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

func (bc *Blockchain) FindSpendableOutputs(address string, amount uint64) (uint64, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	unspentTXs := bc.FindUnspentTransactions(address)
	accumulated := uint64(0)

Work:
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.Hash)

		for outIdx, outProto := range tx.Outputs {
			out := TxOutput{outProto}
			if out.CanBeUnlockedWith(address) && accumulated < amount {
				accumulated += out.Value
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOutputs
}

func (bc *Blockchain) FindUnspentTransactions(address string) []Transaction {
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, txProto := range block.Transactions {
			tx := Transaction{txProto}
			txID := hex.EncodeToString(tx.Hash)

		Outputs:
			for outIdx, outProto := range tx.Outputs {
				out := TxOutput{outProto}
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}

				if out.CanBeUnlockedWith(address) {
					unspentTXs = append(unspentTXs, tx)
				}
			}

			if tx.IsCoinbase() == false {
				for _, inProto := range tx.Inputs {
					in := TxInput{inProto}
					if in.CanUnlockOutputWith(address) {
						inTxID := hex.EncodeToString(in.PrevTxHash)
						spentTXOs[inTxID] = append(spentTXOs[inTxID], int(in.OutputIndex))
					}
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return unspentTXs
}

func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tip, bc.Db()}

	return bci
}

type BlockchainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

func (i *BlockchainIterator) Next() *Block {
	var block *Block

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encodedBlock := b.Get(i.currentHash)
		block = DeserializeBlock(encodedBlock)

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	i.currentHash = block.PrevHash

	return block
}

func NewGenesisBlock() *Block {
	return NewBlock([]byte{}, []*Transaction{NewCoinbaseTX("Satoshi", "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks")}, []byte("genesis"))
}

func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}

	return result.Bytes()
}

func DeserializeBlock(d []byte) *Block {
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}

	return &block
}
