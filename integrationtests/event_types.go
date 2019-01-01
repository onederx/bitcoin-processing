package integrationtests

import (
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

type genericEvent struct {
	Type events.EventType `json:"type"`
	Seq  int              `json:"seq"`
}

type txEvent struct {
	genericEvent
	Data *wallet.TxNotification `json:"data"`
}

type newWalletEvent struct {
	genericEvent
	Data *wallet.Account `json:"data"`
}
