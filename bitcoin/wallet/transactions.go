package wallet

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcutil"
	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/util"
)

// Transaction is processing app's own type for representing transactions.
// It maps to Bitcoin transactions closely, but mapping is not 1-to-1:
// one Bitcoin transaction can produce several Transactions in case it pays
// to several different user's addresses simultaneously or is a transfer from
// one user address to another (in which case it is a withdrawal and a deposit
// at the same time).
// Also, it can be that there is no Bitcoin tx corresponding to Transaction
// instance - this is true for pending and cancelled txns.
// Transaction therefore has its own ID which is a uuid and uniquely identifies
// a Transaction
// Transaction can be stored in DB (Storage interface is made for that) and is
// JSON-serializable
type Transaction struct {
	// Identifier of transaction. For incoming txns it is generated by storage.
	// For outgoing txns, it can be specified by client, otherwise it is
	// generated by api package
	ID uuid.UUID `json:"id"`

	// Hash is a hash of corresponding bitcoin transaction, if one exists.
	// Otherwise it is an empty string
	Hash string `json:"hash"`

	// BlockHash is a hash of Bitcoin block that includes corresponding
	// transaction. If transaction is not yet confirmed (not part of any block),
	// or even not yet broadcasted to network, this is an empty string
	BlockHash string `json:"blockhash"`

	// Confirmations is a number of confirmations this transaction has. Number
	// of confirmations is a number blocks in blockchain since this tx was
	// mined. For tx not yet mined it is 0, for tx that is a part of the most
	// fresh block is is 1, after one more block appears it will become 2 etc
	Confirmations int64 `json:"confirmations"`

	// Address is an address this tx sends money TO - so, for incoming tx it
	// belongs to current wallet, for outgoing tx address is external
	Address string `json:"address"`

	// Direction determines whether this tx is incoming or outgoing, whether it
	// transfers money to or from current wallet
	Direction TransactionDirection `json:"direction"`

	// Status is a enum describing current state of the transaction
	Status TransactionStatus `json:"status"`

	// Amount is amount of money transferred by this tx
	Amount bitcoin.BTCAmount `json:"amount"`

	// Metainfo is structure with some information attached to this tx. For
	// incoming txns, it equals metainfo of account this tx brings money to
	// (it is set on account creation). For withdrawals metainfo can be

	// explicitly set
	Metainfo interface{} `json:"metainfo"`

	// Fee is a fee paid for this tx to bitcoin miner. Only valid for outgoing
	// txns
	Fee bitcoin.BTCAmount `json:"fee"`

	// FeeType is a way fee is calculated (can be set as constant value or
	// calculated as a rate per tx size). Only valid for outgoing txns.

	FeeType bitcoin.FeeType `json:"fee_type"`

	// If tx is a withdrawal to cold storage, this is true. Otherwise false
	ColdStorage bool `json:"cold_storage"`

	fresh                 bool
	reportedConfirmations int64
}

// TransactionDirection is a enum describing whether transaction is incoming
// or outgoing.
type TransactionDirection int

// Possible values of TransactionDirection enum.
// For txns we first see from updates from Bitcoin node, status is
// converted from it's Category field. UnknownDirection is for cases when
// Bitcoin node reported some unexpected category. This may happen if we
// suddenly start mining bitcoins and create a coinbase transaction for our
// wallet.
// InvalidDirection is for cases when direction is converted from other types
// and invalid value of source type is provided.
const (
	IncomingDirection TransactionDirection = iota
	OutgoingDirection
	UnknownDirection
	InvalidDirection
)

// TransactionStatus is a enum describing current state of transaction.
type TransactionStatus int

const (
	// NewTransaction is a status for txns that have been broadcasted to
	// Bitcoin network, but not yet mined (have 0 confirmations)
	NewTransaction TransactionStatus = iota

	// ConfirmedTransaction is a status received by transaction that has at
	// least 1 confirmation, but still less than maximum number of confirmations
	// (this number is set by config param 'transaction.max_confirmations' and
	// is 6 by default)
	ConfirmedTransaction

	// FullyConfirmedTransaction is a status received by transaction that
	// has maximum number of confirmations or more. Such txns are considered
	// fully trusted and updates on them are not further checked by processing
	// app
	FullyConfirmedTransaction

	// PendingTransaction is an outgoing transaction for which there is not
	// enough confirmed balance to send. There still may be enough money to
	// send it after some unconfirmed incoming txns are confirmed, in which case
	// processing app will automatically send such tx (and it's status will
	// change)
	PendingTransaction

	// PendingColdStorageTransaction is an outgoing transaction for which
	// there won't be enough balance to send even if all incoming txns are
	// confirmed. Additional money should be sent to current wallet in order to
	// fund such tx
	PendingColdStorageTransaction

	// PendingManualConfirmationTransaction is a withdrawal which is waiting to
	// be manually confirmed. Withdrawals of amounts higher than a certain value
	// (set by config parameter wallet.min_withdraw_without_manual_confirmation)
	// automatically become pending manual confirmation. By default ALL
	// withdrawals will require manual confirmation. Such tx can be confirmed
	// by making API request to /confirm
	PendingManualConfirmationTransaction

	// CancelledTransaction is a status tx receives when it is cancelled.
	// Pending tx can be cancelled by a call to /cancel_pending. Such txns
	// can be requested from DB with /get_transactions, but not processed in
	// any other way by processing app
	CancelledTransaction

	// InvalidTransaction is a status value generated when converting status
	// from other type and value of source type is invalid
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

// TransactionDirectionFromString converts string representation of
// TransactionDirection to enum value.
func TransactionDirectionFromString(txDirectionStr string) (TransactionDirection, error) {
	td, ok := stringToTransactionDirectionMap[txDirectionStr]
	if !ok {
		return InvalidDirection, errors.New(
			"Invalid transaction direction: " + txDirectionStr,
		)
	}
	return td, nil
}

// TransactionStatusFromString converts string representation of
// TransactionStatus to enum value.
func TransactionStatusFromString(txStatusStr string) (TransactionStatus, error) {
	ts, ok := stringToTransactionStatusMap[txStatusStr]
	if !ok {
		return InvalidTransaction, errors.New(
			"Invalid transaction status: " + txStatusStr,
		)
	}
	return ts, nil
}

// MarshalJSON serializes TransactionDirection to a JSON value. Resulting value
// is simply a string representation of direction
func (td TransactionDirection) MarshalJSON() ([]byte, error) {
	return []byte("\"" + td.String() + "\""), nil
}

// UnmarshalJSON deserializes TransactionDirection from JSON. Resulting value is
// mapped from string representation of tx direction
func (td *TransactionDirection) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*td, err = TransactionDirectionFromString(j)
	return err
}

// MarshalJSON serializes TransactionStatus to a JSON value. Resulting value
// is simply a string representation of status
func (ts TransactionStatus) MarshalJSON() ([]byte, error) {
	return []byte("\"" + ts.String() + "\""), nil
}

// UnmarshalJSON deserializes TransactionStatus from JSON. Resulting value is
// mapped from string representation of tx status
func (ts *TransactionStatus) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*ts, err = TransactionStatusFromString(j)
	return err
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

// ToCoinpaymentsLikeType converts direction to coinpayments-like type:
// "deposit" for incoming, "withdrawal" for outgoing
func (td TransactionDirection) ToCoinpaymentsLikeType() string {
	switch td {
	case IncomingDirection:
		return "deposit"
	case OutgoingDirection:
		return "withdrawal"
	default:
		return "unknown"
	}
}

// ToCoinpaymentsLikeCode converts status to numeric code in a
// coinpayments-like fashion: final status (FullyConfirmedTransaction)
// should become 100, other status codes should be less
func (ts TransactionStatus) ToCoinpaymentsLikeCode() int {
	switch {
	case ts == FullyConfirmedTransaction:
		return 100
	case int(ts) < 100:
		return int(ts)
	default:
		return 99
	}
}
