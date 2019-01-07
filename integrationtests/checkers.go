package integrationtests

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
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

func (tx *txTestData) mineOrFail(t *testing.T, e *testEnvironment) {
	blockHash, err := e.mineTx(tx.hash)
	if err != nil {
		t.Fatalf("Failed to mine tx %s into blockchain: %v", tx.hash, err)
	}
	tx.blockHash = blockHash
	tx.confirmations++
}

type testTxCollection []*txTestData

func (tc testTxCollection) mineOrFail(t *testing.T, e *testEnvironment) {
	var hashes []string
	for _, tx := range tc {
		hashes = append(hashes, tx.hash)
	}
	blockHash, err := e.mineMultipleTxns(hashes)
	if err != nil {
		t.Fatalf("Failed to mine txns %v into blockchain: %v", hashes, err)
	}
	for _, tx := range tc {
		tx.blockHash = blockHash
		tx.confirmations++
	}
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

func checkRequiredFromColdStorage(t *testing.T, e *testEnvironment, balance bitcoin.BTCAmount) {
	required, err := e.processingClient.GetRequiredFromColdStorage()

	if err != nil {
		t.Fatalf("Failed request amount required from cold storage from processing: %v", err)
	}

	if got, want := required, balance; got != want {
		t.Errorf("Expected that amount required from cold storage will be %s "+
			"but got %s", want, got)
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

func checkConfirmedTxNotificationFields(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
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
}

func checkFullyConfirmedTxNotificationFields(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkConfirmedTxNotificationFields(t, n, tx)
	if got, want := n.StatusCode, 100; got != want {
		t.Errorf("Expected status code %d for fully confirmed tx, instead got %d", want, got)
	}
	if got, want := n.StatusStr, wallet.FullyConfirmedTransaction.String(); got != want {
		t.Errorf("Expected status name %s for fully confirmed tx, instead got %s", want, got)
	}
}

func checkPartiallyConfirmedTxNotificationFields(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkConfirmedTxNotificationFields(t, n, tx)
	if !(n.StatusCode > 0 && n.StatusCode < 100) {
		t.Errorf("Expected status code for partially confirmed tx to be more "+
			"than 0, but less than 100. Instead got %d", n.StatusCode)
	}
	if got, want := n.StatusStr, wallet.ConfirmedTransaction.String(); got != want {
		t.Errorf("Expected status name %s for confirmed tx, instead got %s", want, got)
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

func checkNotificationFieldsForPartiallyConfirmedDeposit(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForDeposit(t, n, tx)
	checkPartiallyConfirmedTxNotificationFields(t, n, tx)
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

func checkNotificationFieldsForPartiallyConfirmedWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForWithdraw(t, n, tx)
	checkPartiallyConfirmedTxNotificationFields(t, n, tx)
}

func checkNotificationFieldsForFullyConfirmedClientWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForFullyConfirmedWithdraw(t, n, tx)
	checkNotificationFieldsForClientWithdraw(t, n, tx)
}

func checkNotificationFieldsForPartiallyConfirmedClientWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForPartiallyConfirmedWithdraw(t, n, tx)
	checkNotificationFieldsForClientWithdraw(t, n, tx)
}

func checkNotificationFieldsForAnyPendingWithdraw(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForWithdraw(t, n, tx)
	checkUnconfirmedTxNotificationFields(t, n, tx)
	checkNotificationFieldsForClientWithdraw(t, n, tx)

	if got, want := n.ID, tx.id; got != want {
		t.Errorf("Expected that tx id in withdraw notification data be %s, "+
			"but got %s", n.ID, tx.id)
	}

	if n.Hash != "" {
		t.Errorf("Expected pending tx to have no bitcoin tx hash, but tx "+
			"hash is %s", n.Hash)
	}

	if n.StatusCode >= 100 {
		t.Errorf("Status code for pending tx should be less than 100, but it "+
			"is %d", n.StatusCode)
	}
}

func checkNotificationFieldsForWithdrawPendingManualConfirmation(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForAnyPendingWithdraw(t, n, tx)

	if got, want := n.StatusStr, wallet.PendingManualConfirmationTransaction.String(); got != want {
		t.Errorf("Expected status name %s for tx, instead got %s", want, got)
	}
}

func checkNotificationFieldsForWithdrawPending(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForAnyPendingWithdraw(t, n, tx)

	if got, want := n.StatusStr, wallet.PendingTransaction.String(); got != want {
		t.Errorf("Expected status name %s for tx, instead got %s", want, got)
	}
}

func checkNotificationFieldsForWithdrawPendingColdStorage(t *testing.T, n *wallet.TxNotification, tx *txTestData) {
	checkNotificationFieldsForAnyPendingWithdraw(t, n, tx)

	if got, want := n.StatusStr, wallet.PendingColdStorageTransaction.String(); got != want {
		t.Errorf("Expected status name %s for tx, instead got %s", want, got)
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

func checkNewWithdrawTransactionNotificationAndEvent(t *testing.T, env *testEnvironment,
	notification *wallet.TxNotification, event *events.NotificationWithSeq,
	tx *txTestData, clientBalance, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	checkNotificationFieldsForNewClientWithdraw(t, notification, tx)

	if got, want := notification.StatusStr, wallet.NewTransaction.String(); got != want {
		t.Errorf("Expected status name %s for new outgoing tx, instead got %s", want, got)
	}

	tx.hash = notification.Hash

	if got, want := event.Type, events.NewOutgoingTxEvent; got != want {
		t.Errorf("Expected type of event for fresh successful withdraw "+
			"to be %s, instead got %s", want, got)
	}

	data := event.Data.(*wallet.TxNotification)

	checkNotificationFieldsForNewClientWithdraw(t, data, tx)

	if got, want := data.StatusStr, wallet.NewTransaction.String(); got != want {
		t.Errorf("Expected status name %s for new outgoing tx, instead got %s", want, got)
	}

	if got, want := data.Hash, tx.hash; got != want {
		t.Errorf("Expected bitcoin tx hash to be equal in http and "+
			"websocket notification, instead they are %s %s",
			tx.hash, data.Hash)
	}
	checkClientBalanceBecame(t, env, clientBalance,
		expectedClientBalanceAfterWithdraw)
}
