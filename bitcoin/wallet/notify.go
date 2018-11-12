package wallet

import (
	"github.com/shopspring/decimal"

	"github.com/onederx/bitcoin-processing/events"
)

type txNotification struct {
	Transaction
	StatusCode int    `json:"status"`
	StatusStr  string `json:"status_name"`
	IpnType    string `json:"ipn_type"`
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
	IpnId      string `json:"ipn_id"`
	Fee        string `json:"fee"`
}

func (w *Wallet) NotifyTransaction(eventType events.EventType, tx Transaction) {
	w.eventBroker.Notify(eventType, txNotification{
		Transaction: tx,
		StatusCode:  tx.Status.ToCoinpaymentsLikeCode(),
		StatusStr:   tx.Status.String(),
		IpnType:     tx.Direction.ToCoinpaymentsLikeType(),
		Currency:    "BTC",
		Amount:      decimal.New(int64(tx.Amount), -8).String(),
		IpnId:       tx.Id.String(),
		Fee:         decimal.New(int64(tx.Amount), -8).String(),
	})
}
