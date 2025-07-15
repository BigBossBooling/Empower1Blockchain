package core

import (
	"testing"
)

func TestBlockchain(t *testing.T) {
	bc := NewBlockchain()
	defer bc.Db().Close()

	if bc == nil {
		t.Fatal("NewBlockchain() returned nil")
	}

	if bc.tip == nil {
		t.Fatal("Blockchain tip is nil")
	}
}

func TestAddBlock(t *testing.T) {
	bc := NewBlockchain()
	defer bc.Db().Close()

	bc.AddBlock([]*Transaction{})

	if bc.tip == nil {
		t.Fatal("Blockchain tip is nil")
	}
}

func TestNewTransaction(t *testing.T) {
	bc := NewBlockchain()
	defer bc.Db().Close()

	cbTx := NewCoinbaseTX("alice", "")
	bc.AddBlock([]*Transaction{cbTx})

	tx := NewTransaction("alice", "bob", 10, bc)

	if tx == nil {
		t.Fatal("NewTransaction() returned nil")
	}
}
