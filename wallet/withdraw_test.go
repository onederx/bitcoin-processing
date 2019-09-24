package wallet

import (
	"testing"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/nodeapi"
	"github.com/onederx/bitcoin-processing/events"
	settingstestutil "github.com/onederx/bitcoin-processing/settings/testutil"
	"github.com/onederx/bitcoin-processing/wallet/types"
)

const (
	testTxIDStr = "8e70c722-45fe-445c-93c6-262f49bbc710"
)

var (
	testTxID = uuid.Must(uuid.FromString(testTxIDStr))
)

type nodeAPIBalanceAndAddressMock struct {
	nodeapi.NodeAPI
}

func (n *nodeAPIBalanceAndAddressMock) GetConfirmedAndUnconfirmedBalance() (uint64, uint64, error) {
	return 1, 0, nil
}

func (n *nodeAPIBalanceAndAddressMock) CreateNewAddress() (string, error) {
	return testAddress, nil
}

type loggingEventBrokerMock struct {
	events.EventBroker

	log []*events.Notification
}

func (l *loggingEventBrokerMock) Notify(eventType events.EventType, data interface{}) error {
	l.log = append(l.log, &events.Notification{
		Type: eventType,
		Data: data,
	})
	return nil
}

func (l *loggingEventBrokerMock) flushEvents() {
	l.log = l.log[:0]
}

func (l *loggingEventBrokerMock) assertExpectedEvents(t *testing.T, expected []*events.Notification) {
	if got, want := len(l.log), len(expected); got != want {
		t.Fatalf("Expected to have %d events, but have only %d", want, got)
	}

	for i := 0; i < len(expected); i++ {
		got, want := l.log[i], expected[i]

		if got.Type != want.Type {
			t.Fatalf("Expected type of %d'th event to be %s, got %s",
				i, got.Type, want.Type)
		}

		assertTxEventDataEqual(t, i, got.Data.(types.TxNotification), want.Data.(types.TxNotification))
	}
}

func (l *loggingEventBrokerMock) SendNotifications() {}

func assertTxEventDataEqual(t *testing.T, i int, got, want types.TxNotification) {
	if got.ID == uuid.Nil {
		t.Errorf("Id of tx in event is unexpectedly nil")
	}
	if want.ID != uuid.Nil && got.ID != want.ID {
		t.Errorf("Expected id of %d'th event tx data to be %s, got %s",
			i, want.ID, got.ID)
	}
	if got.Address != want.Address {
		t.Errorf("Expected address of %d'th event tx data to be %s, got %s",
			i, want.Address, got.Address)
	}
	if got.Amount != want.Amount {
		t.Errorf("Expected amount of %d'th event tx data to be %s, got %s",
			i, want.Amount, got.Amount)
	}
	if got.ColdStorage != want.ColdStorage {
		t.Errorf("Expected cold storage flag of %d'th event tx data to be %t, got %t",
			i, want.ColdStorage, got.ColdStorage)
	}
	if got.Confirmations != want.Confirmations {
		t.Errorf("Expected amount of confirmations of %d'th event tx data to be %d, got %d",
			i, want.Confirmations, got.Confirmations)
	}
	if got.Direction != want.Direction {
		t.Errorf("Expected direction of %d'th event tx data to be %s, got %s",
			i, want.Direction, got.Direction)
	}
	if got.Status != want.Status {
		t.Errorf("Expected status of %d'th event tx data to be %s, got %s",
			i, want.Status, got.Status)
	}
	if got.FeeType != want.FeeType {
		t.Errorf("Expected fee type of %d'th event tx data to be %s, got %s",
			i, want.FeeType, got.FeeType)
	}
	if got.Fee != want.Fee {
		t.Errorf("Expected fee amount of %d'th event tx data to be %s, got %s",
			i, want.Fee, got.Fee)
	}
}

func TestInternalWithdraw(t *testing.T) {
	const feeType = bitcoin.FixedFee

	var (
		amount = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))
		fee    = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0001"))
	)
	s := &settingstestutil.SettingsMock{
		Data: map[string]interface{}{
			"transaction.max_confirmations": 1,
		},
	}
	n := &nodeAPIBalanceAndAddressMock{}
	e := &loggingEventBrokerMock{}
	ws := NewStorage(nil)
	w := NewWallet(s, n, e, ws)

	acct, _ := w.CreateAccount(nil)

	tx := &types.Transaction{
		ID:                    testTxID,
		Confirmations:         0,
		Address:               acct.Address,
		Direction:             types.OutgoingDirection,
		Amount:                amount,
		Metainfo:              nil,
		Fee:                   fee,
		FeeType:               feeType,
		ColdStorage:           false,
		Fresh:                 true,
		ReportedConfirmations: -1,
	}

	e.flushEvents()
	err := w.internalWithdrawBetweenOurAccounts(tx, acct)

	if err != nil {
		t.Fatal(err)
	}

	expectedEvents := []*events.Notification{
		&events.Notification{
			Type: events.NewOutgoingTxEvent,
			Data: types.TxNotification{
				Transaction: types.Transaction{
					ID:            testTxID,
					Address:       testAddress,
					Amount:        amount,
					Fee:           fee,
					FeeType:       feeType,
					Direction:     types.OutgoingDirection,
					Confirmations: 0,
					Status:        types.NewTransaction,
					ColdStorage:   false,
				},
			},
		},
		&events.Notification{
			Type: events.OutgoingTxConfirmedEvent,
			Data: types.TxNotification{
				Transaction: types.Transaction{
					ID:            testTxID,
					Address:       testAddress,
					Amount:        amount,
					Fee:           fee,
					FeeType:       feeType,
					Direction:     types.OutgoingDirection,
					Confirmations: 1,
					Status:        types.FullyConfirmedTransaction,
					ColdStorage:   false,
				},
			},
		},
		&events.Notification{
			Type: events.NewIncomingTxEvent,
			Data: types.TxNotification{
				Transaction: types.Transaction{
					Address:       testAddress,
					Amount:        amount - fee,
					Fee:           fee,
					FeeType:       feeType,
					Direction:     types.IncomingDirection,
					Confirmations: 0,
					Status:        types.NewTransaction,
					ColdStorage:   false,
				},
			},
		},
		&events.Notification{
			Type: events.IncomingTxConfirmedEvent,
			Data: types.TxNotification{
				Transaction: types.Transaction{
					Address:       testAddress,
					Amount:        amount - fee,
					Fee:           fee,
					FeeType:       feeType,
					Direction:     types.IncomingDirection,
					Confirmations: 1,
					Status:        types.FullyConfirmedTransaction,
					ColdStorage:   false,
				},
			},
		},
	}

	e.assertExpectedEvents(t, expectedEvents)
}

func TestInternalWithdrawToColdStorage(t *testing.T) {
	const feeType = bitcoin.FixedFee

	var (
		amount = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))
	)
	s := &settingstestutil.SettingsMock{
		Data: map[string]interface{}{
			"transaction.max_confirmations": 1,
		},
	}
	n := &nodeAPIBalanceAndAddressMock{}
	e := &loggingEventBrokerMock{}
	ws := NewStorage(nil)
	w := NewWallet(s, n, e, ws)

	acct, _ := w.CreateAccount(nil)

	tx := &types.Transaction{
		ID:                    testTxID,
		Confirmations:         0,
		Address:               acct.Address,
		Direction:             types.OutgoingDirection,
		Amount:                amount,
		Metainfo:              nil,
		Fee:                   0,
		FeeType:               feeType,
		ColdStorage:           true,
		Fresh:                 true,
		ReportedConfirmations: -1,
	}

	e.flushEvents()
	err := w.internalWithdrawBetweenOurAccounts(tx, acct)

	if err != nil {
		t.Fatal(err)
	}

	expectedEvents := []*events.Notification{
		&events.Notification{
			Type: events.NewIncomingTxEvent,
			Data: types.TxNotification{
				Transaction: types.Transaction{
					Address:       testAddress,
					Amount:        amount,
					Fee:           0,
					FeeType:       feeType,
					Direction:     types.IncomingDirection,
					Confirmations: 0,
					Status:        types.NewTransaction,
					ColdStorage:   true,
				},
			},
		},
		&events.Notification{
			Type: events.IncomingTxConfirmedEvent,
			Data: types.TxNotification{
				Transaction: types.Transaction{
					Address:       testAddress,
					Amount:        amount,
					Fee:           0,
					FeeType:       feeType,
					Direction:     types.IncomingDirection,
					Confirmations: 1,
					Status:        types.FullyConfirmedTransaction,
					ColdStorage:   true,
				},
			},
		},
	}

	e.assertExpectedEvents(t, expectedEvents)
}

func TestInternalWithdrawWhenFeeExceedsAmount(t *testing.T) {
	const feeType = bitcoin.FixedFee

	var (
		amount = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0001"))
		fee    = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.001"))
	)
	s := &settingstestutil.SettingsMock{
		Data: map[string]interface{}{
			"transaction.max_confirmations": 1,
		},
	}
	n := &nodeAPIBalanceAndAddressMock{}
	e := &loggingEventBrokerMock{}
	ws := NewStorage(nil)
	w := NewWallet(s, n, e, ws)

	acct, _ := w.CreateAccount(nil)

	tx := &types.Transaction{
		ID:                    testTxID,
		Confirmations:         0,
		Address:               acct.Address,
		Direction:             types.OutgoingDirection,
		Amount:                amount,
		Metainfo:              nil,
		Fee:                   fee,
		FeeType:               feeType,
		ColdStorage:           false,
		Fresh:                 true,
		ReportedConfirmations: -1,
	}

	e.flushEvents()
	err := w.internalWithdrawBetweenOurAccounts(tx, acct)

	if err == nil {
		t.Error("Expected internal withdraw to return error, but it did not")
	}

	if len(e.log) > 0 {
		t.Errorf("Expected broken internal withdraw to generate no events, "+
			"but it generated %d events", len(e.log))
	}
}
