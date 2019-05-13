package wallet

import (
	"encoding/json"

	"github.com/onederx/bitcoin-processing/events"
)

type TxNotification struct {
	Transaction
	StatusCode int    `json:"status"`
	StatusStr  string `json:"status_name"`
	IpnType    string `json:"ipn_type"`
	Currency   string `json:"currency"`
	IpnID      string `json:"ipn_id"`
	Seq        int    `json:"seq,omitempty"`
}

func init() {
	txEvents := []events.EventType{
		events.NewIncomingTxEvent,
		events.IncomingTxConfirmedEvent,
		events.NewOutgoingTxEvent,
		events.OutgoingTxConfirmedEvent,
		events.PendingStatusUpdatedEvent,
		events.PendingTxCancelledEvent,
	}
	for _, et := range txEvents {
		events.RegisterNotificationUnmarshaler(et, func(b []byte) (interface{}, error) {
			var notification TxNotification

			err := json.Unmarshal(b, &notification)
			return &notification, err
		})
	}
}

// NotifyTransaction is used to send a notification about a new or updated
// transaction. In fact it is a wrapper around EventBroker.Notify that
// transforms notification into coinpayments-like format. Before moving to that
// format, EventBroker.Notify was called directly and this may become the case
// in future after API change (Transaction itself is JSON-serializable and can
// act as data field of event)
func (w *Wallet) NotifyTransaction(eventType events.EventType, tx Transaction) error {
	return w.eventBroker.Notify(eventType, TxNotification{
		Transaction: tx,
		StatusCode:  tx.Status.ToCoinpaymentsLikeCode(),
		StatusStr:   tx.Status.String(),
		IpnType:     tx.Direction.ToCoinpaymentsLikeType(),
		Currency:    "BTC",
		IpnID:       tx.ID.String(),
	})
}
