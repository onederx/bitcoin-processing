package types

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
	for txStatus, txStatusStr := range transactionStatusToStringMap {
		stringToTransactionStatusMap[txStatusStr] = txStatus
	}
}
