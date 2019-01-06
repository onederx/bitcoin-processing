package integrationtests

import (
	"testing"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

func testWithdraw(t *testing.T, env *testEnvironment) {
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)

	defaultWithdrawRequest := wallet.WithdrawRequest{
		Address: withdrawAddress,
		Amount:  withdrawAmountSmall,
		Fee:     withdrawFee,
	}

	clientBalance, err := env.getClientBalance()

	if err != nil {
		t.Fatal(err)
	}

	if clientBalance.Balance != clientBalance.BalanceWithUnconf {
		t.Fatalf("Expected client balance to have no uncofirmed part. "+
			"Instead, confirmed and full balances are %s %s",
			clientBalance.Balance, clientBalance.BalanceWithUnconf)
	}

	runSubtest(t, "WithoutManualConfirmation", func(t *testing.T) {
		expectedClientBalanceAfterWithdraw := clientBalance.Balance + withdrawAmountSmall - withdrawFee
		resp, err := env.processingClient.Withdraw(&defaultWithdrawRequest)
		if err != nil {
			t.Fatal(err)
		}
		checkClientWithdrawRequest(t, resp, &defaultWithdrawRequest)

		tx := &txTestData{
			id:      resp.ID,
			address: withdrawAddress,
			amount:  withdrawAmountSmall,
			fee:     withdrawFee,
		}

		runSubtest(t, "NewTransaction", func(t *testing.T) {
			testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})

		wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")) // deposit - withdraw: 0.5 - 0.05
		checkBalance(t, env, wantBalance, wantBalance)

		tx.blockHash, err = env.mineTx(tx.hash)

		if err != nil {
			t.Fatalf("Failed to mine tx into blockchain: %v", err)
		}

		tx.confirmations = 1

		runSubtest(t, "ConfirmedTransaction", func(t *testing.T) {
			testWithdrawFullyConfirmed(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})
	})

	clientBalance, err = env.getClientBalance()

	if err != nil {
		t.Fatal(err)
	}

	if clientBalance.Balance != clientBalance.BalanceWithUnconf {
		t.Fatalf("Expected client balance to have no uncofirmed part. "+
			"Instead, confirmed and full balances are %s %s",
			clientBalance.Balance, clientBalance.BalanceWithUnconf)
	}

	runSubtest(t, "WithManualConfirmation", func(t *testing.T) {
		withdrawAmountBig := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.15"))
		withdrawRequest := defaultWithdrawRequest
		withdrawRequest.Amount = withdrawAmountBig

		resp, err := env.processingClient.Withdraw(&withdrawRequest)
		if err != nil {
			t.Fatal(err)
		}
		checkClientWithdrawRequest(t, resp, &withdrawRequest)

		tx := &txTestData{
			id:      resp.ID,
			address: withdrawAddress,
			amount:  withdrawAmountBig,
			fee:     withdrawFee,
		}

		runSubtest(t, "NewTransactionNotConfirmedYet", func(t *testing.T) {
			testWithdrawTransactionPendingManualConfirmation(t, env, tx,
				bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")))
		})
		expectedClientBalanceAfterWithdraw := clientBalance.Balance + withdrawAmountBig - withdrawFee
		runSubtest(t, "ManuallyConfirmTransaction", func(t *testing.T) {
			err := env.processingClient.Confirm(tx.id)

			if err != nil {
				t.Fatalf("Failed to confirm tx: %v", err)
			}
			testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})

		// deposit - small withdraw - big withdraw: 0.5 - 0.05 - 0.15
		wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.3"))
		checkBalance(t, env, wantBalance, wantBalance)

		tx.blockHash, err = env.mineTx(tx.hash)

		if err != nil {
			t.Fatalf("Failed to mine tx into blockchain: %v", err)
		}

		tx.confirmations = 1

		runSubtest(t, "ConfirmedTransaction", func(t *testing.T) {
			testWithdrawFullyConfirmed(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})

		runSubtest(t, "CancelInsteadOfConfirming", func(t *testing.T) {
			resp, err := env.processingClient.Withdraw(&withdrawRequest)
			if err != nil {
				t.Fatal(err)
			}
			checkClientWithdrawRequest(t, resp, &withdrawRequest)

			tx := &txTestData{
				id:      resp.ID,
				address: withdrawAddress,
				amount:  withdrawAmountBig,
				fee:     withdrawFee,
			}
			testWithdrawTransactionPendingManualConfirmation(t, env, tx,
				wantBalance)
			err = env.processingClient.Cancel(tx.id)

			if err != nil {
				t.Fatalf("Failed to cancel tx pending manual confirmation: %v",
					err)
			}
			notification := env.getNextCallbackNotificationWithTimeout(t)

			checkNotificationFieldsForCancelledWithdrawal(t, notification, tx)

			event := env.websocketListeners[0].getNextMessageWithTimeout(t)

			if got, want := event.Type, events.PendingTxCancelledEvent; got != want {
				t.Errorf("Expected event type to be %s, but got %s", want, got)
			}

			data := event.Data.(*wallet.TxNotification)

			checkNotificationFieldsForCancelledWithdrawal(t, data, tx)

			// same balance as before, because this tx is cancelled
			checkBalance(t, env, wantBalance, wantBalance)
		})
	})

	runSubtest(t, "InsufficientFunds", func(t *testing.T) {
		testWithdrawInsufficientFunds(t, env)
	})

	runSubtest(t, "FixedID", func(t *testing.T) {
		req := defaultWithdrawRequest
		withdrawID := uuid.Must(uuid.FromString("e06ed38b-ff2c-4e3d-885f-135fe6c72625"))
		req.ID = withdrawID
		resp, err := env.processingClient.Withdraw(&req)
		if err != nil {
			t.Fatal(err)
		}
		checkClientWithdrawRequest(t, resp, &req)
		if resp.ID != withdrawID {
			t.Errorf("Expected resulting withdraw id to be equal to "+
				"requested one %s, but got %s", withdrawID, resp.ID)
		}
		notification := env.getNextCallbackNotificationWithTimeout(t)
		if notification.ID != withdrawID {
			t.Errorf("Expected withdraw id in http notification to be equal "+
				"to requested one %s, but got %s", withdrawID, notification.ID)
		}
		event := env.websocketListeners[0].getNextMessageWithTimeout(t)
		data := event.Data.(*wallet.TxNotification)
		if data.ID != req.ID {
			t.Errorf("Expected withdraw id in http notification to be equal "+
				"to requested one %s, but got %s", withdrawID, data.ID)
		}
		env.mineTx(notification.Hash)
		// skip corresponding notifications
		env.getNextCallbackNotificationWithTimeout(t)
		env.websocketListeners[0].getNextMessageWithTimeout(t)
	})
}

func testWithdrawInsufficientFunds(t *testing.T, env *testEnvironment) {

}

func testWithdrawNewTransaction(t *testing.T, env *testEnvironment, tx *txTestData, clientBalance *api.BalanceInfo, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForNewClientWithdraw(t, notification, tx)

	if got, want := notification.StatusStr, wallet.NewTransaction.String(); got != want {
		t.Errorf("Expected status name %s for new outgoing tx, instead got %s", want, got)
	}

	tx.hash = notification.Hash

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

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
	checkClientBalanceBecame(t, env, clientBalance.Balance,
		expectedClientBalanceAfterWithdraw)
}

func testWithdrawPartiallyConfirmed(t *testing.T, env *testEnvironment, tx *txTestData, clientBalance *api.BalanceInfo, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	notification := env.getNextCallbackNotificationWithTimeout(t)
	checkNotificationFieldsForPartiallyConfirmedClientWithdraw(t, notification, tx)
	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

	if got, want := event.Type, events.OutgoingTxConfirmedEvent; got != want {
		t.Errorf("Expected type of event for confirmed successful withdraw "+
			"to be %s, instead got %s", want, got)
	}
	data := event.Data.(*wallet.TxNotification)
	checkNotificationFieldsForPartiallyConfirmedClientWithdraw(t, data, tx)
	checkClientBalanceBecame(t, env,
		expectedClientBalanceAfterWithdraw,
		expectedClientBalanceAfterWithdraw)
}

func testWithdrawFullyConfirmed(t *testing.T, env *testEnvironment, tx *txTestData, clientBalance *api.BalanceInfo, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	notification := env.getNextCallbackNotificationWithTimeout(t)
	checkNotificationFieldsForFullyConfirmedClientWithdraw(t, notification, tx)
	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

	if got, want := event.Type, events.OutgoingTxConfirmedEvent; got != want {
		t.Errorf("Expected type of event for confirmed successful withdraw "+
			"to be %s, instead got %s", want, got)
	}
	data := event.Data.(*wallet.TxNotification)
	checkNotificationFieldsForFullyConfirmedClientWithdraw(t, data, tx)
	checkClientBalanceBecame(t, env,
		expectedClientBalanceAfterWithdraw,
		expectedClientBalanceAfterWithdraw)
}

func testWithdrawTransactionPendingManualConfirmation(t *testing.T, env *testEnvironment, tx *txTestData, ourOldBalance bitcoin.BTCAmount) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForWithdrawPendingManualConfirmation(t, notification, tx)

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

	if got, want := event.Type, events.PendingStatusUpdatedEvent; got != want {
		t.Errorf("Expected event type to be %s, but got %s", want, got)
	}

	data := event.Data.(*wallet.TxNotification)

	checkNotificationFieldsForWithdrawPendingManualConfirmation(t, data, tx)

	// same balance as before, because this tx is not confirmed yet
	checkBalance(t, env, ourOldBalance, ourOldBalance)
}

func testWithdrawSeveralConfirmations(t *testing.T, env *testEnvironment, neededConfirmations int) {
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)

	defaultWithdrawRequest := wallet.WithdrawRequest{
		Address: withdrawAddress,
		Amount:  withdrawAmountSmall,
		Fee:     withdrawFee,
	}

	clientBalance, err := env.getClientBalance()

	if err != nil {
		t.Fatal(err)
	}

	if clientBalance.Balance != clientBalance.BalanceWithUnconf {
		t.Fatalf("Expected client balance to have no uncofirmed part. "+
			"Instead, confirmed and full balances are %s %s",
			clientBalance.Balance, clientBalance.BalanceWithUnconf)
	}

	expectedClientBalanceAfterWithdraw := clientBalance.Balance + withdrawAmountSmall - withdrawFee
	resp, err := env.processingClient.Withdraw(&defaultWithdrawRequest)
	if err != nil {
		t.Fatal(err)
	}
	checkClientWithdrawRequest(t, resp, &defaultWithdrawRequest)

	tx := &txTestData{
		id:      resp.ID,
		address: withdrawAddress,
		amount:  withdrawAmountSmall,
		fee:     withdrawFee,
	}

	runSubtest(t, "NewTransaction", func(t *testing.T) {
		testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
	})

	wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")) // deposit - withdraw: 0.5 - 0.05
	checkBalance(t, env, wantBalance, wantBalance)

	tx.blockHash, err = env.mineTx(tx.hash)

	if err != nil {
		t.Fatalf("Failed to mine tx into blockchain: %v", err)
	}

	tx.confirmations = 1

	runSubtest(t, "Confirmation", func(t *testing.T) {
		testWithdrawPartiallyConfirmed(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)

		for i := 2; i < neededConfirmations; i++ {
			_, err := generateBlocks(env.regtest["node-miner"].nodeAPI, 1)
			if err != nil {
				t.Fatal(err)
			}
			tx.confirmations = int64(i)
			testWithdrawPartiallyConfirmed(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		}
		_, err := generateBlocks(env.regtest["node-miner"].nodeAPI, 1)
		if err != nil {
			t.Fatal(err)
		}
		tx.confirmations++

		testWithdrawFullyConfirmed(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
	})
}

func testWithdrawMultiple(t *testing.T, env *testEnvironment) {
	// TODO
}
