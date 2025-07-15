package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/empower1/empower1/internal/aiml"
	"github.com/empower1/empower1/proto"
)

type Transaction struct {
	*proto.Transaction
}

func (tx *Transaction) SetHash() {
	var encoded bytes.Buffer
	var hash [32]byte

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	hash = sha256.Sum256(encoded.Bytes())
	tx.Hash = hash[:]
}

func (tx *Transaction) GetHash() string {
	return hex.EncodeToString(tx.Hash)
}

type TxInput struct {
	*proto.TxInput
}

type TxOutput struct {
	*proto.TxOutput
}

func NewCoinbaseTX(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Reward to '%s'", to)
	}

	txin := &TxInput{&proto.TxInput{PrevTxHash: []byte{}, OutputIndex: 0, ScriptSig: []byte(data)}}
	txout := &TxOutput{&proto.TxOutput{Value: 100, ScriptPubKey: []byte(to)}}
	tx := &Transaction{&proto.Transaction{Inputs: []*proto.TxInput{txin.TxInput}, Outputs: []*proto.TxOutput{txout.TxOutput}}}
	tx.SetHash()

	return tx
}

func (in *TxInput) CanUnlockOutputWith(unlockingData string) bool {
	return bytes.Compare(in.ScriptSig, []byte(unlockingData)) == 0
}

func (out *TxOutput) CanBeUnlockedWith(unlockingData string) bool {
	return bytes.Compare(out.ScriptPubKey, []byte(unlockingData)) == 0
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].PrevTxHash) == 0
}

func NewTransaction(from, to string, amount uint64, bc *Blockchain) *Transaction {
	var inputs []*proto.TxInput
	var outputs []*proto.TxOutput

	acc, validOutputs := bc.FindSpendableOutputs(from, amount)

	if acc < amount {
		log.Panic("ERROR: Not enough funds")
	}

	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			log.Panic(err)
		}

		for _, out := range outs {
			input := &proto.TxInput{PrevTxHash: txID, OutputIndex: uint32(out), ScriptSig: []byte(from)}
			inputs = append(inputs, input)
		}
	}

	outputs = append(outputs, &proto.TxOutput{Value: amount, ScriptPubKey: []byte(to)})
	if acc > amount {
		outputs = append(outputs, &proto.TxOutput{Value: acc - amount, ScriptPubKey: []byte(from)}) // a change
	}

	tax := uint64(float64(amount) * aiml.GetTaxRate([]byte(from)))
	if tax > 0 {
		outputs = append(outputs, &proto.TxOutput{Value: tax, ScriptPubKey: []byte("tax")})
	}

	tx := &Transaction{&proto.Transaction{Inputs: inputs, Outputs: outputs}}
	tx.SetHash()

	return tx
}
