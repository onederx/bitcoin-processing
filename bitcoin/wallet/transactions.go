package wallet

import (
	"errors"
	"log"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcutil"
	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/util"
)

type TransactionDirection int

const (
	IncomingDirection TransactionDirection = iota
	OutgoingDirection
	UnknownDirection
	InvalidDirection
)

type TransactionStatus int

const (
	NewTransaction TransactionStatus = iota
	ConfirmedTransaction
	FullyConfirmedTransaction
	PendingTransaction
	PendingColdStorageTransaction
	PendingManualConfirmationTransaction
	CancelledTransaction
	InvalidTransaction
)

var transactionDirectionToStringMap = map[TransactionDirection]string{
	IncomingDirection: "incoming",
	OutgoingDirection: "outgoing",
	UnknownDirection:  "unknown",
}

var stringToTransactionDirectionMap = make(map[string]TransactionDirection)

var transactionStatusToStringMap = map[TransactionStatus]string{
	NewTransaction:                       "new",
	ConfirmedTransaction:                 "confirmed",
	FullyConfirmedTransaction:            "fully-confirmed",
	PendingTransaction:                   "pending",
	PendingColdStorageTransaction:        "pending-cold-storage",
	PendingManualConfirmationTransaction: "pending-manual-confirmation",
	CancelledTransaction:                 "cancelled",
}

var stringToTransactionStatusMap = make(map[string]TransactionStatus)

func init() {
	for txDirection, txDirectionStr := range transactionDirectionToStringMap {
		stringToTransactionDirectionMap[txDirectionStr] = txDirection
	}
	for txStatus, txStatusStr := range transactionStatusToStringMap {
		stringToTransactionStatusMap[txStatusStr] = txStatus
	}
}

func (td TransactionDirection) String() string {
	tdStr, ok := transactionDirectionToStringMap[td]
	if !ok {
		return "invalid"
	}
	return tdStr
}

func (ts TransactionStatus) String() string {
	tsStr, ok := transactionStatusToStringMap[ts]
	if !ok {
		return "invalid"
	}
	return tsStr
}

func TransactionDirectionFromString(txDirectionStr string) (TransactionDirection, error) {
	td, ok := stringToTransactionDirectionMap[txDirectionStr]
	if !ok {
		return InvalidDirection, errors.New(
			"Invalid transaction direction: " + txDirectionStr,
		)
	}
	return td, nil
}

func TransactionStatusFromString(txStatusStr string) (TransactionStatus, error) {
	ts, ok := stringToTransactionStatusMap[txStatusStr]
	if !ok {
		return InvalidTransaction, errors.New(
			"Invalid transaction status: " + txStatusStr,
		)
	}
	return ts, nil
}

func (td TransactionDirection) MarshalJSON() ([]byte, error) {
	return []byte("\"" + td.String() + "\""), nil
}

func (ts TransactionStatus) MarshalJSON() ([]byte, error) {
	return []byte("\"" + ts.String() + "\""), nil
}

type Transaction struct {
	ID            uuid.UUID            `json:"id"`
	Hash          string               `json:"hash"`
	BlockHash     string               `json:"blockhash"`
	Confirmations int64                `json:"confirmations"`
	Address       string               `json:"address"`
	Direction     TransactionDirection `json:"direction"`
	Status        TransactionStatus    `json:"status"`
	Amount        bitcoin.BTCAmount    `json:"amount"`
	Metainfo      interface{}          `json:"metainfo"`
	Fee           bitcoin.BTCAmount    `json:"fee"`
	FeeType       bitcoin.FeeType      `json:"fee_type"`
	ColdStorage   bool                 `json:"cold_storage"`

	fresh                 bool
	reportedConfirmations int64
}

func (tx *Transaction) update(other *Transaction) {
	if tx.Hash != "" && tx.Hash != other.Hash {
		panic("Tx update called for transaction with other hash")
	}
	tx.Hash = other.Hash
	tx.BlockHash = other.BlockHash
	tx.Confirmations = other.Confirmations
	tx.Status = other.Status
}

func (tx *Transaction) updateFromFullTxInfo(other *btcjson.GetTransactionResult) {
	if tx.Hash != "" && tx.Hash != other.TxID {
		panic("Tx update called for transaction with other hash")
	}
	tx.BlockHash = other.BlockHash
	tx.Confirmations = other.Confirmations
}

func newTransaction(btcNodeTransaction *btcjson.ListTransactionsResult) *Transaction {
	var direction TransactionDirection
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

	var amount bitcoin.BTCAmount
	btcutilAmount, err := btcutil.NewAmount(btcNodeTransaction.Amount)
	if err != nil {
		log.Printf(
			"Error: failed to convert amount %v to btcutil amount for tx %s."+
				"Amount is probably invalid. Full tx %#v",
			btcNodeTransaction.Amount,
			btcNodeTransaction.TxID,
			btcNodeTransaction,
		)
		amount = 0
	} else {
		amount = bitcoin.BTCAmount(util.Abs64(int64(btcutilAmount)))
	}

	return &Transaction{
		Hash:                  btcNodeTransaction.TxID,
		BlockHash:             btcNodeTransaction.BlockHash,
		Confirmations:         btcNodeTransaction.Confirmations,
		Address:               btcNodeTransaction.Address,
		Direction:             direction,
		Status:                NewTransaction,
		Amount:                amount,
		ColdStorage:           false,
		fresh:                 true,
		reportedConfirmations: -1,
	}
}

func (td TransactionDirection) ToCoinpaymentsLikeType() string {
	// convert direction to coinpayments-like type:
	// "deposit" for incoming, "withdrawal" for outgoing
	switch td {
	case IncomingDirection:
		return "deposit"
	case OutgoingDirection:
		return "withdrawal"
	default:
		return "unknown"
	}
}

func (ts TransactionStatus) ToCoinpaymentsLikeCode() int {
	// convert status to numeric code in a coinpayments-like fashion:
	// final status (FullyConfirmedTransaction) should become 100,
	// other status codes should be less
	switch {
	case ts == FullyConfirmedTransaction:
		return 100
	case int(ts) < 100:
		return int(ts)
	default:
		return 99
	}
}
