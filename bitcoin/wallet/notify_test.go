package wallet

import (
	"reflect"
	"testing"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/events"
)

type eventBrokerNotifyCheckMock struct {
	eventBrokerMock
	notifyChecker func(eventType events.EventType, data interface{})
}

func (e *eventBrokerNotifyCheckMock) Notify(eventType events.EventType, data interface{}) {
	e.notifyChecker(eventType, data)
}

func TestNotify(t *testing.T) {
	tests := []struct {
		idStr         string
		hash          string
		address       string
		amount        bitcoin.BTCAmount
		blockHash     string
		confirmations int64
		direction     TransactionDirection
		status        TransactionStatus
		metainfo      interface{}
		eventType     events.EventType
		ipnType       string
		statusCode    int
		statusStr     string
		fee           bitcoin.BTCAmount
		feeType       bitcoin.FeeType
	}{
		{
			idStr:         "da76b24c-5d61-4958-87e5-050824f85039",
			hash:          "289577beb72068bb70c96b3384c4b719d6c1c71eb662dcea18ecc6045a6c016d",
			address:       "2NG1f7FNtDiWEAyKKCEgCT1fxzM7Sd9L16k",
			amount:        bitcoin.BTCAmount(100000000),
			blockHash:     "",
			confirmations: 0,
			direction:     IncomingDirection,
			status:        NewTransaction,
			metainfo:      testMetainfo,
			eventType:     events.NewIncomingTxEvent,
			ipnType:       "deposit",
			statusCode:    0,
			statusStr:     NewTransaction.String(),
		},
		{
			idStr:         "1a5f3fb4-81f2-43e0-b661-77e4def4218f",
			hash:          "cba261e6059b23a3e84c1d077b1863a198bc83066d66c876852af7bcfe62136b",
			address:       "2MtSxNhH535tfzJD1Eg7yXC1FCR9vZgkv47",
			amount:        bitcoin.BTCAmount(231400000),
			blockHash:     "0fe641842f5eef2be8b90d71bbb9fe8b1be97adbceff04447c5f4809a0765e68",
			confirmations: 1,
			direction:     OutgoingDirection,
			status:        FullyConfirmedTransaction,
			metainfo:      testMetainfo,
			eventType:     events.OutgoingTxConfirmedEvent,
			ipnType:       "withdrawal",
			statusCode:    100,
			statusStr:     FullyConfirmedTransaction.String(),
			fee:           bitcoin.BTCAmount(40000),
			feeType:       bitcoin.FixedFee,
		},
	}

	for _, test := range tests {
		s := &settingsMock{}
		txid, err := uuid.FromString(test.idStr)

		if err != nil {
			panic("Bad test uuid " + test.idStr)
		}

		brokerMock := &eventBrokerNotifyCheckMock{
			notifyChecker: func(eventType events.EventType, data interface{}) {
				if got, want := eventType, test.eventType; got != want {
					t.Errorf("Expected event type %v for notification, got %v", want, got)
				}
				notification := data.(TxNotification)
				if got, want := notification.Address, test.address; got != want {
					t.Errorf("Expected address %s in notification, got %s", want, got)
				}
				if got, want := notification.Amount, test.amount; got != want {
					t.Errorf("Expected amount %v in notification, got %v", want, got)
				}
				if got, want := notification.BlockHash, test.blockHash; got != want {
					t.Errorf("Expected block hash %v in notification, got %v", want, got)
				}
				if got, want := notification.Confirmations, test.confirmations; got != want {
					t.Errorf("Expected confirmations %d in notification, got %d", want, got)
				}
				if got, want := notification.Currency, "BTC"; got != want {
					t.Errorf("Expected currency %s in notification, got %s", want, got)
				}
				if got, want := notification.Direction, test.direction; got != want {
					t.Errorf("Expected direction %s in notification, got %s", want, got)
				}
				if got, want := notification.Hash, test.hash; got != want {
					t.Errorf("Expected hash %s in notification, got %s", want, got)
				}
				if got, want := notification.ID, txid; got != want {
					t.Errorf("Expected id %s in notification, got %s", want, got)
				}
				if got, want := notification.IpnID, test.idStr; got != want {
					t.Errorf("Expected ipn_id %s in notification, got %s", want, got)
				}
				if got, want := notification.IpnType, test.ipnType; got != want {
					t.Errorf("Expected ipn_type %s in notification, got %s", want, got)
				}
				if !reflect.DeepEqual(notification.Metainfo, test.metainfo) {
					t.Errorf("Expected metainfo %v in notification, got %v",
						testMetainfo, notification.Metainfo)
				}
				if got, want := notification.StatusCode, test.statusCode; got != want {
					t.Errorf("Expected status %d in notification, got %d", want, got)
				}
				if got, want := notification.StatusStr, test.statusStr; got != want {
					t.Errorf("Expected status_name %s in notification, got %s", want, got)
				}
				if got, want := notification.Fee, test.fee; got != want {
					t.Errorf("Expected fee %v in notification, got %v", want, got)
				}
				if got, want := notification.FeeType, test.feeType; got != want {
					t.Errorf("Expected fee type %v in notification, got %v", want, got)
				}
			},
		}

		w := NewWallet(s, nil, brokerMock, NewStorage("memory", s))
		w.NotifyTransaction(test.eventType, Transaction{
			ID:            txid,
			BlockHash:     test.blockHash,
			Hash:          test.hash,
			Confirmations: test.confirmations,
			Address:       test.address,
			Direction:     test.direction,
			Status:        test.status,
			Amount:        test.amount,
			Metainfo:      test.metainfo,
			Fee:           test.fee,
			FeeType:       test.feeType,
		})
	}
}
