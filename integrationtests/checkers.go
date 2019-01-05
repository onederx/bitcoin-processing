package integrationtests

import (
	"encoding/json"
	"fmt"
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
	fee       bitcoin.BTCAmount
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

func checkClientBalanceBecame(t *testing.T, e *testEnvironment, balance, balanceWithUnconf bitcoin.BTCAmount) {
	waitForEventOrFailTest(t, func() error {
		clientBalance, err := e.getClientBalance()
		if err != nil {
			return err
		}
		if clientBalance.Balance != balance {
			return fmt.Errorf("Expected confirmed client balance to be %s, "+
				"but it it %s", balance, clientBalance.Balance)
		}
		if clientBalance.BalanceWithUnconf != balanceWithUnconf {
			return fmt.Errorf("Expected client balance including unconfirmed to be %s, "+
				"but it it %s", balanceWithUnconf, clientBalance.BalanceWithUnconf)
		}
		return nil
	})
}

func checkTxNotificationFields(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	if got, want := n.Amount, tx.amount; got != want {
		t.Errorf("Incorrect amount for tx: expected %s, got %s", want, got)
	}
	if got, want := n.Currency, "BTC"; got != want {
		t.Errorf("Currency should always be '%s', instead got '%s'", want, got)
	}
	txIDStr := n.ID.String()
	if txIDStr == "" || n.IpnID != txIDStr {
		t.Errorf("Expected tx id and 'ipn_id' field to be equal and nonempty "+
			"instead they were '%s' and '%s'", txIDStr, n.IpnID)
	}
	compareMetainfo(t, n.Metainfo, tx.metainfo)
}

func checkUnconfirmedTxNotificationFields(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	if got, want := n.BlockHash, ""; got != want {
		t.Errorf("Expected that block hash for new tx will be empty, instead got %s", got)
	}
	if got, want := n.Confirmations, int64(0); got != want {
		t.Errorf("Expected 0 confirmations for new tx, instead got %d", got)
	}
}

func checkNewTxNotificationFields(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkUnconfirmedTxNotificationFields(t, n, tx)
	if got, want := n.StatusCode, 0; got != want {
		t.Errorf("Expected status code 0 for new tx, instead got %d", got)
	}
}

func checkFullyConfirmedTxNotificationFields(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	if got, want := n.ID, tx.id; got != want {
		t.Errorf("Expected that tx id will be %s, instead got %s", want, got)
	}
	if got, want := n.Hash, tx.hash; got != want {
		t.Errorf("Expected that bitcoin tx hash will be %s, instead got %s", want, got)
	}
	if got, want := n.BlockHash, tx.blockHash; got != want {
		t.Errorf("Expected that block hash will be %s, instead got %s", want, got)
	}
	if got, want := n.Confirmations, tx.confirmations; got != want {
		t.Errorf("Expected %d confirmations for confirmed tx, instead got %d", want, got)
	}
	if got, want := n.StatusCode, 100; got != want {
		t.Errorf("Expected status code %d for fully confirmed tx, instead got %d", want, got)
	}
	if got, want := n.StatusStr, wallet.FullyConfirmedTransaction.String(); got != want {
		t.Errorf("Expected status name %s for new incoming tx, instead got %s", want, got)
	}
}

func checkNotificationFieldsForDeposit(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkTxNotificationFields(t, n, tx)
	if got, want := n.Address, tx.address; got != want {
		t.Errorf("Incorrect address for deposit: expected %s, got %s", want, got)
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
	if n.ColdStorage != false {
		t.Errorf("Cold storage flag should be false for any incoming tx, instead it was true")
	}
}

func checkNotificationFieldsForNewDeposit(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForDeposit(t, n, tx)
	checkNewTxNotificationFields(t, n, tx)
	if got, want := n.StatusStr, wallet.NewTransaction.String(); got != want {
		t.Errorf("Expected status name %s for new incoming tx, instead got %s", want, got)
	}
}

func checkNotificationFieldsForFullyConfirmedDeposit(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForDeposit(t, n, tx)
	checkFullyConfirmedTxNotificationFields(t, n, tx)
}

func checkWithdrawRequest(t *testing.T, gotRequest *wallet.WithdrawRequest, wantRequest *wallet.WithdrawRequest) {
	if got, want := gotRequest.Amount, wantRequest.Amount; got != want {
		t.Errorf("Expected resulting amount to equal requested one %s, but got %s",
			want, got)
	}
	if got, want := gotRequest.Fee, wantRequest.Fee; got != want {
		t.Errorf("Expected resulting fee to equal requested one %s, but got %s",
			want, got)
	}
	if gotRequest.ID == uuid.Nil {
		t.Errorf("Expected resulting tx id to be non-nil")
	}
}

func checkClientWithdrawRequest(t *testing.T, gotRequest *wallet.WithdrawRequest, wantRequest *wallet.WithdrawRequest) {
	checkWithdrawRequest(t, gotRequest, wantRequest)
	if got, want := gotRequest.Address, wantRequest.Address; got != want {
		t.Errorf("Expected resulting address to equal requested one %s, but got %s",
			want, got)
	}
}

func checkCSWithdrawRequest(t *testing.T, gotRequest *wallet.WithdrawRequest, wantRequest *wallet.WithdrawRequest, defaultCSAddress string) {
	checkWithdrawRequest(t, gotRequest, wantRequest)
	if wantRequest.Address == "" {
		if got, want := gotRequest.Address, defaultCSAddress; got != want {
			t.Errorf("Expected resulting address to equal default cold storage address %s, but got %s",
				want, got)
		}
	} else {
		if got, want := gotRequest.Address, wantRequest.Address; got != want {
			t.Errorf("Expected resulting address to equal requested one %s, but got %s",
				want, got)
		}

	}
}

func checkNotificationFieldsForWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkTxNotificationFields(t, n, tx)
	if got, want := n.Direction, wallet.OutgoingDirection; got != want {
		t.Errorf("Incorrect direction for withdraw: expected %s, got %s", want, got)
	}
	if got, want := n.IpnType, "withdrawal"; got != want {
		t.Errorf("Expected 'ipn_type' field to be '%s', instead got '%s'", want, got)
	}
	if got, want := n.Fee, tx.fee; got != want {
		t.Errorf("Expected fee field to be %s, instead got %s", want, got)
	}
}

func checkNotificationFieldsForNewWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForWithdraw(t, n, tx)
	checkNewTxNotificationFields(t, n, tx)
}

func checkNotificationFieldsForClientWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	if got, want := n.Address, tx.address; got != want {
		t.Errorf("Incorrect address for regular withdraw: expected %s, got %s", want, got)
	}
	if n.ColdStorage != false {
		t.Errorf("Cold storage flag should be false for regular withdraw, instead it was true")
	}
}

func checkNotificationFieldsForNewClientWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForNewWithdraw(t, n, tx)
	checkNotificationFieldsForClientWithdraw(t, n, tx)

	if got, want := n.ID, tx.id; got != want {
		t.Errorf("Expected that tx id in withdraw notification data be %s, "+
			"but got %s", n.ID, tx.id)
	}
}

func checkNotificationFieldsForFullyConfirmedWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForWithdraw(t, n, tx)
	checkFullyConfirmedTxNotificationFields(t, n, tx)
}

func checkNotificationFieldsForFullyConfirmedClientWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForFullyConfirmedWithdraw(t, n, tx)
	checkNotificationFieldsForClientWithdraw(t, n, tx)
}

func checkNotificationFieldsForWithdrawPendingManualConfirmation(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForWithdraw(t, n, tx)
	checkUnconfirmedTxNotificationFields(t, n, tx)
	checkNotificationFieldsForClientWithdraw(t, n, tx)

	if got, want := n.ID, tx.id; got != want {
		t.Errorf("Expected that tx id in withdraw notification data be %s, "+
			"but got %s", n.ID, tx.id)
	}

	if got, want := n.StatusStr, wallet.PendingManualConfirmationTransaction.String(); got != want {
		t.Errorf("Expected status name %s for tx, instead got %s", want, got)
	}

	if n.Hash != "" {
		t.Errorf("Expected tx pending manual confirmation to have no "+
			"bitcoin tx hash, but tx hash is %s", n.Hash)
	}

	if n.StatusCode >= 100 {
		t.Errorf("Status code for pending tx should be less than 100, but it "+
			"is %d", n.StatusCode)
	}
}

func checkNotificationFieldsForCancelledWithdrawal(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForWithdraw(t, n, tx)
	checkUnconfirmedTxNotificationFields(t, n, tx)
	checkNotificationFieldsForClientWithdraw(t, n, tx)

	if got, want := n.ID, tx.id; got != want {
		t.Errorf("Expected that tx id in withdraw notification data be %s, "+
			"but got %s", n.ID, tx.id)
	}

	if got, want := n.StatusStr, wallet.CancelledTransaction.String(); got != want {
		t.Errorf("Expected status name %s for tx, instead got %s", want, got)
	}

	if n.Hash != "" {
		t.Errorf("Expected tx pending manual confirmation to have no "+
			"bitcoin tx hash, but tx hash is %s", n.Hash)
	}

	if n.StatusCode >= 100 {
		t.Errorf("Status code for cancelled tx should be less than 100, but it "+
			"is %d", n.StatusCode)
	}
}
