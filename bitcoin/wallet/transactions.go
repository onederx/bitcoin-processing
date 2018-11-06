package wallet

import (
	"errors"
	"log"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcutil"
	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/util"
)

type TransactionDirection int

const (
	IncomingDirection TransactionDirection = iota
	OutgoingDirection
	UnknownDirection
	InvalidDirection
)

var transactionDirectionToStringMap map[TransactionDirection]string = map[TransactionDirection]string{
	IncomingDirection: "incoming",
	OutgoingDirection: "outgoing",
	UnknownDirection:  "unknown",
}

var stringToTransactionDirectionMap map[string]TransactionDirection = make(map[string]TransactionDirection)

func init() {
	for txDirection, txDirectionStr := range transactionDirectionToStringMap {
		stringToTransactionDirectionMap[txDirectionStr] = txDirection
	}
}

func (td TransactionDirection) String() string {
	tdStr, ok := transactionDirectionToStringMap[td]
	if ok {
		return tdStr
	}
	return "invalid"
}

func TransactionDirectionFromString(txDirectionStr string) (TransactionDirection, error) {
	td, ok := stringToTransactionDirectionMap[txDirectionStr]
	if ok {
		return td, nil
	}
	return InvalidDirection, errors.New(
		"Invalid transaction direction: " + txDirectionStr,
	)
}

func (td TransactionDirection) MarshalJSON() ([]byte, error) {
	return []byte("\"" + td.String() + "\""), nil
}

type Transaction struct {
	Id            uuid.UUID            `json:"id"`
	Hash          string               `json:"hash"`
	BlockHash     string               `json:"blockhash"`
	Confirmations int64                `json:"confirmations"`
	Address       string               `json:"address"`
	Direction     TransactionDirection `json:"direction"`
	Amount        uint64               `json:"amount"` // satoshis
	Metainfo      interface{}          `json:"metainfo"`

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
	var satoshis uint64
	if btcNodeTransaction.Category == "receive" {
		direction = IncomingDirection
	} else if btcNodeTransaction.Category == "send" {
		direction = OutgoingDirection
	} else {
		log.Printf(
			"Warning: unexpected transaction category %s for tx %s",
			btcNodeTransaction.Category,
			btcNodeTransaction.TxID,
		)
		direction = UnknownDirection
	}

	if direction == IncomingDirection && btcNodeTransaction.Amount < 0 {
		log.Printf(
			"Warning: unexpected amount %f for transaction %s. Amount for "+
				"incoming transaction should be nonnegative. Transaction: %#v",
			btcNodeTransaction.Amount,
			btcNodeTransaction.TxID,
			btcNodeTransaction,
		)
	} else if direction == OutgoingDirection && btcNodeTransaction.Amount > 0 {
		log.Printf(
			"Warning: unexpected amount %f for transaction %s. Amount for "+
				"outgoing transaction should not be positive. Transaction: %#v",
			btcNodeTransaction.Amount,
			btcNodeTransaction.TxID,
			btcNodeTransaction,
		)
	}

	btcutilAmount, err := btcutil.NewAmount(btcNodeTransaction.Amount)

	if err != nil {
		log.Printf(
			"Error: failed to convert amount %v to btcutil amount for tx %s."+
				"Amount is probably invalid. Full tx %#v",
			btcNodeTransaction.Amount,
			btcNodeTransaction.TxID,
			btcNodeTransaction,
		)
		satoshis = 0
	} else {
		satoshis = uint64(util.Abs64(int64(btcutilAmount)))
	}

	return &Transaction{
		Hash:                  btcNodeTransaction.TxID,
		BlockHash:             btcNodeTransaction.BlockHash,
		Confirmations:         btcNodeTransaction.Confirmations,
		Address:               btcNodeTransaction.Address,
		Direction:             direction,
		Amount:                satoshis,
		reportedConfirmations: -1,
	}
}
