package wallet

import (
	"log"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/satori/go.uuid"
)

type TransactionDirection string

const (
	DIRECTION_INCOMING TransactionDirection = "incoming"
	DIRECTION_OUTGOING                      = "outgoing"
	DIRECTION_UNKNOWN                       = "unknown"
)

type Transaction struct {
	Id            uuid.UUID            `json:"id"`
	Hash          string               `json:"hash"`
	BlockHash     string               `json:"blockhash"`
	Confirmations int64                `json:"confirmations"`
	Address       string               `json:"address"`
	Direction     TransactionDirection `json:"direction"`

	reportedConfirmations int64
}

func (tx *Transaction) update(other *Transaction) {
	if tx.Hash != other.Hash {
		panic("Tx update called for transaction with other hash")
	}
	tx.BlockHash = other.BlockHash
	tx.Confirmations = other.Confirmations
}

func (tx *Transaction) updateFromFullTxInfo(other *btcjson.GetTransactionResult) {
	if tx.Hash != other.TxID {
		panic("Tx update called for transaction with other hash")
	}
	tx.BlockHash = other.BlockHash
	tx.Confirmations = other.Confirmations
}

func newTransaction(btcNodeTransaction *btcjson.ListTransactionsResult) *Transaction {
	var direction TransactionDirection
	if btcNodeTransaction.Category == "receive" {
		direction = DIRECTION_INCOMING
	} else if btcNodeTransaction.Category == "send" {
		direction = DIRECTION_OUTGOING
	} else {
		log.Printf(
			"Warning: unexpected transaction category %s for tx %s",
			btcNodeTransaction.Category,
			btcNodeTransaction.TxID,
		)
		direction = DIRECTION_UNKNOWN
	}

	return &Transaction{
		Hash:                  btcNodeTransaction.TxID,
		BlockHash:             btcNodeTransaction.BlockHash,
		Confirmations:         btcNodeTransaction.Confirmations,
		Address:               btcNodeTransaction.Address,
		Direction:             direction,
		reportedConfirmations: -1,
	}
}
