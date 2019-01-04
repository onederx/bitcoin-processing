package integrationtests

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
)

type txTestData struct {
	id        uuid.UUID
	address   string
	amount    bitcoin.BTCAmount
	hash      string
	blockHash string
	metainfo  interface{}

	confirmations int64
}

func compareMetainfo(t *testing.T, got, want interface{}) {
	gotJSON, err := json.MarshalIndent(got, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal metainfo %#v to JSON for comparison: %s", got, err)
	}
	wantJSON, err := json.MarshalIndent(want, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal metainfo %#v to JSON for comparison: %s", want, err)
	}
	gotJSONStr, wantJSONStr := string(gotJSON), string(wantJSON)

	var gotUnified, wantUnified interface{}

	err = json.Unmarshal(gotJSON, &gotUnified)

	if err != nil {
		t.Fatalf("Failed to unmarshal metainfo %s back from JSON for comparison: %s", gotJSONStr, err)
	}

	err = json.Unmarshal(wantJSON, &wantUnified)

	if err != nil {
		t.Fatalf("Failed to unmarshal metainfo %s back from JSON for comparison: %s", wantJSONStr, err)
	}

	if !reflect.DeepEqual(gotUnified, wantUnified) {
		t.Fatalf("Unexpected metainfo. Got %s, wanted %s", gotJSONStr, wantJSONStr)
	}
}

func checkBalance(t *testing.T, e *testEnvironment, balance, balanceWithUnconf bitcoin.BTCAmount) {
	balanceInfo, err := e.processingClient.GetBalance()

	if err != nil {
		t.Fatalf("Failed to get balance from processing: %v", err)
	}

	if got, want := balanceInfo.Balance, balance; got != want {
		t.Errorf("Wrong confirmed wallet balance: expected %s, got %s", want, got)
	}

	if got, want := balanceInfo.BalanceWithUnconf, balanceWithUnconf; got != want {
		t.Errorf("Wrong wallet balance including unconfirmed: expected %s, got %s", want, got)
	}
}

func checkNotificationFieldsForDeposit(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	if got, want := n.Address, tx.address; got != want {
		t.Errorf("Incorrect address for deposit: expected %s, got %s", want, got)
	}
	if got, want := n.Amount, tx.amount; got != want {
		t.Errorf("Incorrect amount for deposit: expected %s, got %s", want, got)
	}
	if got, want := n.Direction, wallet.IncomingDirection; got != want {
		t.Errorf("Incorrect direction for deposit: expected %s, got %s", want, got)
	}
	if got, want := n.Hash, tx.hash; got != want {
		t.Errorf("Incorrect tx hash for deposit: expected %s, got %s", want, got)
	}
	if got, want := n.IpnType, "deposit"; got != want {
		t.Errorf("Expected 'ipn_type' field to be '%s', instead got '%s'", want, got)
	}
	if got, want := n.Currency, "BTC"; got != want {
		t.Errorf("Currency should always be '%s', instead got '%s'", want, got)
	}
	txIDStr := n.ID.String()
	if txIDStr == "" || n.IpnID != txIDStr {
		t.Errorf("Expected tx id and 'ipn_id' field to be equal and nonempty "+
			"instead they were '%s' and '%s'", txIDStr, n.IpnID)
	}
	if n.ColdStorage != false {
		t.Errorf("Cold storage flag should be empty for any incoming tx, instead it was true")
	}
	compareMetainfo(t, n.Metainfo, tx.metainfo)
}

func checkNotificationFieldsForNewDeposit(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForDeposit(t, n, tx)
	if got, want := n.BlockHash, ""; got != want {
		t.Errorf("Expected that block hash for new tx will be empty, instead got %s", got)
	}
	if got, want := n.Confirmations, int64(0); got != want {
		t.Errorf("Expected 0 confirmations for new tx, instead got %d", got)
	}
	if got, want := n.StatusCode, 0; got != want {
		t.Errorf("Expected status code 0 for new incoming tx, instead got %d", got)
	}
	if got, want := n.StatusStr, wallet.NewTransaction.String(); got != want {
		t.Errorf("Expected status name %s for new incoming tx, instead got %s", want, got)
	}
}

func checkNotificationFieldsForFullyConfirmedDeposit(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForDeposit(t, n, tx)
	if got, want := n.BlockHash, tx.blockHash; got != want {
		t.Errorf("Expected that block hash will be %s, instead got %s", want, got)
	}
	if got, want := n.Confirmations, tx.confirmations; got != want {
		t.Errorf("Expected %d confirmations for confirmed tx, instead got %d", want, got)
	}
	if got, want := n.StatusCode, 100; got != want {
		t.Errorf("Expected status code %d for confirmed tx, instead got %d", want, got)
	}
	if got, want := n.StatusStr, wallet.FullyConfirmedTransaction.String(); got != want {
		t.Errorf("Expected status name %s for new incoming tx, instead got %s", want, got)
	}
}
