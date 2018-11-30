package wallet

import (
	"github.com/onederx/bitcoin-processing/events"
)

type txNotification struct {
	Transaction
	StatusCode int    `json:"status"`
	StatusStr  string `json:"status_name"`
	IpnType    string `json:"ipn_type"`
	Currency   string `json:"currency"`
	IpnId      string `json:"ipn_id"`
}

func (w *Wallet) NotifyTransaction(eventType events.EventType, tx Transaction) {
	w.eventBroker.Notify(eventType, txNotification{
		Transaction: tx,
		StatusCode:  tx.Status.ToCoinpaymentsLikeCode(),
		StatusStr:   tx.Status.String(),
		IpnType:     tx.Direction.ToCoinpaymentsLikeType(),
		Currency:    "BTC",
		IpnId:       tx.Id.String(),
	})
}
