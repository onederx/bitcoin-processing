package wallet

import (
	"github.com/satori/go.uuid"
)

type Transaction struct {
	id            uuid.UUID
	Hash          string
	BlockHash     string
	Confirmations int64
	Address       string
}

func (tx Transaction) update(other *Transaction) {
	if tx.Hash != other.Hash {
		panic("Tx update called for transaction with other hash")
	}
	tx.BlockHash = other.BlockHash
	tx.Confirmations = other.Confirmations
}
