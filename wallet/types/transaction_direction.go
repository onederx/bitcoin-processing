package types

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

var transactionDirectionToStringMap = map[TransactionDirection]string{
	IncomingDirection: "incoming",
	OutgoingDirection: "outgoing",
	UnknownDirection:  "unknown",
}

var stringToTransactionDirectionMap = make(map[string]TransactionDirection)

func init() {
	for txDirection, txDirectionStr := range transactionDirectionToStringMap {
		stringToTransactionDirectionMap[txDirectionStr] = txDirection
	}
}
