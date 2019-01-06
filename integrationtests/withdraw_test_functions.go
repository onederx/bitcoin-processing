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

	clientBalance := getStableClientBalanceOrFail(t, env)

	runSubtest(t, "WithoutManualConfirmation", func(t *testing.T) {
		expectedClientBalanceAfterWithdraw := clientBalance + withdrawAmountSmall - withdrawFee
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
			testWithdrawFullyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
		})
	})

	clientBalance = getStableClientBalanceOrFail(t, env)

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
				bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")), true)
		})
		expectedClientBalanceAfterWithdraw := clientBalance + withdrawAmountBig - withdrawFee
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
			testWithdrawFullyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
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
				wantBalance, true)
			err = env.processingClient.Cancel(tx.id)

			if err != nil {
				t.Fatalf("Failed to cancel tx pending manual confirmation: %v",
					err)
			}

			testWithdrawTransactionCancelled(t, env, tx)

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
	testNames := map[bool]string{false: "PayMissing", true: "Cancel"}
	for _, doCancel := range []bool{false, true} {
		name := testNames[doCancel]
		runSubtest(t, name, func(t *testing.T) {
			runSubtest(t, "PendingIncomingTxConfirmation", func(t *testing.T) {
				testWithdrawInsufficientFundsPending(t, env, doCancel)
			})
			runSubtest(t, "PendingColdStorage", func(t *testing.T) {
				testWithdrawInsufficientFundsPendingColdStorage(t, env, doCancel)
			})
		})
	}
}

func testWithdrawInsufficientFundsPending(t *testing.T, env *testEnvironment, cancel bool) {
	ourBalance := getStableBalanceOrFail(t, env)
	withdrawAmountTooBig := ourBalance + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))
	unconfirmedIncome := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("2"))

	clientAccount, err := env.processingClient.NewWallet(nil)

	if err != nil {
		t.Fatal(err)
	}
	// skip corresponding notification
	env.websocketListeners[0].getNextMessageWithTimeout(t)
	unconfirmedIncomeTxHash, err := env.regtest["node-client"].nodeAPI.SendWithPerKBFee(
		clientAccount.Address, unconfirmedIncome, depositFee, false,
	)

	// skip corresponding notifications
	env.getNextCallbackNotificationWithTimeout(t)
	env.websocketListeners[0].getNextMessageWithTimeout(t)

	checkBalance(t, env, ourBalance, ourBalance+unconfirmedIncome)

	tx := testMakePendingWithdraw(t, env, withdrawAmountTooBig)

	clientBalance := getStableClientBalanceOrFail(t, env)

	runSubtest(t, "GetTransactionsPending", func(t *testing.T) {
		testGetTransactionsTxFoundByStatus(t, env, tx.id, wallet.PendingTransaction)
	})

	if cancel {
		err := env.processingClient.Cancel(tx.id)
		if err != nil {
			t.Fatal(err)
		}
		testWithdrawTransactionCancelled(t, env, tx)
		checkBalance(t, env, ourBalance, ourBalance+unconfirmedIncome)
	}

	_, err = env.mineTx(unconfirmedIncomeTxHash)

	if err != nil {
		t.Fatal(err)
	}

	if cancel {
		// skip notifications from incoming tx confirmation
		env.getNextCallbackNotificationWithTimeout(t)
		env.websocketListeners[0].getNextMessageWithTimeout(t)
		ourBalanceAfterIncome := ourBalance + unconfirmedIncome
		checkBalance(t, env, ourBalanceAfterIncome, ourBalanceAfterIncome)
		return
	}

	notifications, wsEvents := collectNotificationsAndEvents(t, env, 2)

	withdrawNotification := findNotificationForTxOrFail(t, notifications, tx)
	withdrawEvent := findEventWithTypeOrFail(t, wsEvents, events.NewOutgoingTxEvent)

	clientBalanceAfterWithdraw := clientBalance + withdrawAmountTooBig - withdrawFee

	checkNewWithdrawTransactionNotificationAndEvent(t, env, withdrawNotification,
		withdrawEvent, tx, clientBalance, clientBalanceAfterWithdraw)

	blockHash, err := env.mineTx(tx.hash)

	if err != nil {
		t.Fatal(err)
	}

	tx.blockHash = blockHash
	tx.confirmations = 1

	testWithdrawFullyConfirmed(t, env, tx, clientBalanceAfterWithdraw)
}

func testWithdrawInsufficientFundsPendingColdStorage(t *testing.T, env *testEnvironment, cancel bool) {
	ourBalance := getStableBalanceOrFail(t, env)
	overflowAmount := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))
	withdrawAmountTooBig := ourBalance + overflowAmount

	checkRequiredFromColdStorage(t, env, zeroBTC)

	tx := testMakePendingWithdraw(t, env, withdrawAmountTooBig)

	testWithdrawTransactionPendingColdStorage(t, env, tx, ourBalance, true)

	checkRequiredFromColdStorage(t, env, overflowAmount)

	runSubtest(t, "GetTransactionsPendingColdStorage", func(t *testing.T) {
		testGetTransactionsTxFoundByStatus(t, env, tx.id, wallet.PendingColdStorageTransaction)
	})

	if cancel {
		err := env.processingClient.Cancel(tx.id)
		if err != nil {
			t.Fatal(err)
		}
		testWithdrawTransactionCancelled(t, env, tx)
		checkRequiredFromColdStorage(t, env, zeroBTC)
		checkBalance(t, env, ourBalance, ourBalance)
		return
	}

	hsAddress, err := env.processingClient.GetHotStorageAddress()

	if err != nil {
		t.Fatal(err)
	}

	incomeAmount := overflowAmount + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))

	incomeTxHash, err := env.regtest["node-client"].nodeAPI.SendWithPerKBFee(
		hsAddress, incomeAmount, depositFee, false,
	)

	if err != nil {
		t.Fatal(err)
	}

	// now that there are unconfirmed funds that can fund this tx, it becomes
	// pending again
	testWithdrawTransactionPending(t, env, tx, zeroBTC, false)

	runSubtest(t, "GetTransactionsNotPendingColdStorage", func(t *testing.T) {
		testGetTransactionsTxNotFoundByStatus(t, env, tx.id, wallet.PendingColdStorageTransaction)
	})
	runSubtest(t, "GetTransactionsPending", func(t *testing.T) {
		testGetTransactionsTxFoundByStatus(t, env, tx.id, wallet.PendingTransaction)
	})
	checkRequiredFromColdStorage(t, env, zeroBTC)

	_, err = env.mineTx(incomeTxHash)

	if err != nil {
		t.Fatal(err)
	}

	clientBalance := getStableClientBalanceOrFail(t, env)
	clientBalanceAfterWithdraw := clientBalance + withdrawAmountTooBig - withdrawFee

	testWithdrawNewTransaction(t, env, tx, clientBalance, clientBalanceAfterWithdraw)

	blockHash, err := env.mineTx(tx.hash)

	if err != nil {
		t.Fatal(err)
	}

	tx.blockHash = blockHash
	tx.confirmations = 1

	testWithdrawFullyConfirmed(t, env, tx, clientBalanceAfterWithdraw)
}

func testMakePendingWithdraw(t *testing.T, env *testEnvironment, amount bitcoin.BTCAmount) *txTestData {
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)

	resp, err := env.processingClient.Withdraw(&wallet.WithdrawRequest{
		Address: withdrawAddress,
		Amount:  amount,
		Fee:     withdrawFee,
	})

	if err != nil {
		t.Fatal(err)
	}

	// first, this withdraw will wait for manual confirmation
	tx := &txTestData{
		id:      resp.ID,
		address: withdrawAddress,
		amount:  amount,
		fee:     withdrawFee,
	}
	testWithdrawTransactionPendingManualConfirmation(t, env, tx, zeroBTC, false)

	err = env.processingClient.Confirm(tx.id)

	if err != nil {
		t.Fatal(err)
	}

	// alright, now it should become pending
	testWithdrawTransactionPending(t, env, tx, zeroBTC, false)

	return tx
}

func testWithdrawNewTransaction(t *testing.T, env *testEnvironment, tx *txTestData, clientBalance, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	notification := env.getNextCallbackNotificationWithTimeout(t)
	event := env.websocketListeners[0].getNextMessageWithTimeout(t)
	checkNewWithdrawTransactionNotificationAndEvent(t, env, notification,
		event, tx, clientBalance, expectedClientBalanceAfterWithdraw)
}

func testWithdrawPartiallyConfirmed(t *testing.T, env *testEnvironment, tx *txTestData, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
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

func testWithdrawFullyConfirmed(t *testing.T, env *testEnvironment, tx *txTestData, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
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

func testWithdrawTransactionPendingManualConfirmation(t *testing.T, env *testEnvironment, tx *txTestData, ourOldBalance bitcoin.BTCAmount, testBalance bool) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForWithdrawPendingManualConfirmation(t, notification, tx)

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

	if got, want := event.Type, events.PendingStatusUpdatedEvent; got != want {
		t.Errorf("Expected event type to be %s, but got %s", want, got)
	}

	data := event.Data.(*wallet.TxNotification)

	checkNotificationFieldsForWithdrawPendingManualConfirmation(t, data, tx)

	if testBalance {
		// same balance as before, because this tx is not confirmed yet
		checkBalance(t, env, ourOldBalance, ourOldBalance)
	}
}

func testWithdrawTransactionPendingColdStorage(t *testing.T, env *testEnvironment, tx *txTestData, ourOldBalance bitcoin.BTCAmount, testBalance bool) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForWithdrawPendingColdStorage(t, notification, tx)

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

	if got, want := event.Type, events.PendingStatusUpdatedEvent; got != want {
		t.Errorf("Expected event type to be %s, but got %s", want, got)
	}

	data := event.Data.(*wallet.TxNotification)

	checkNotificationFieldsForWithdrawPendingColdStorage(t, data, tx)

	if testBalance {
		// same balance as before, because this tx is not confirmed yet
		checkBalance(t, env, ourOldBalance, ourOldBalance)
	}
}

func testWithdrawTransactionPending(t *testing.T, env *testEnvironment, tx *txTestData, ourOldBalance bitcoin.BTCAmount, testBalance bool) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForWithdrawPending(t, notification, tx)

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

	if got, want := event.Type, events.PendingStatusUpdatedEvent; got != want {
		t.Errorf("Expected event type to be %s, but got %s", want, got)
	}

	data := event.Data.(*wallet.TxNotification)

	checkNotificationFieldsForWithdrawPending(t, data, tx)

	if testBalance {
		// same balance as before, because this tx is not confirmed yet
		checkBalance(t, env, ourOldBalance, ourOldBalance)
	}
}

func testWithdrawTransactionCancelled(t *testing.T, env *testEnvironment, tx *txTestData) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForCancelledWithdrawal(t, notification, tx)

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)

	if got, want := event.Type, events.PendingTxCancelledEvent; got != want {
		t.Errorf("Expected event type to be %s, but got %s", want, got)
	}

	data := event.Data.(*wallet.TxNotification)

	checkNotificationFieldsForCancelledWithdrawal(t, data, tx)
}

func testWithdrawSeveralConfirmations(t *testing.T, env *testEnvironment, neededConfirmations int) {
	clientBalance := getStableClientBalanceOrFail(t, env)
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)

	defaultWithdrawRequest := wallet.WithdrawRequest{
		Address: withdrawAddress,
		Amount:  withdrawAmountSmall,
		Fee:     withdrawFee,
	}

	expectedClientBalanceAfterWithdraw := clientBalance + withdrawAmountSmall - withdrawFee
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
		testWithdrawPartiallyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)

		for i := 2; i < neededConfirmations; i++ {
			_, err := generateBlocks(env.regtest["node-miner"].nodeAPI, 1)
			if err != nil {
				t.Fatal(err)
			}
			tx.confirmations = int64(i)
			testWithdrawPartiallyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
		}
		_, err := generateBlocks(env.regtest["node-miner"].nodeAPI, 1)
		if err != nil {
			t.Fatal(err)
		}
		tx.confirmations++

		testWithdrawFullyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
	})
}

func testGetTransactionsTxFoundByStatus(t *testing.T, env *testEnvironment, txID uuid.UUID, status wallet.TransactionStatus) {
	statusStr := status.String()
	txns, err := env.processingClient.GetTransactions(&api.GetTransactionsFilter{
		Status: statusStr,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, filteredTx := range txns {
		if filteredTx.ID == txID {
			return
		}
	}
	t.Error("GetTransactions API did not return test transaction with "+
		"status %s", statusStr)
}

func testGetTransactionsTxNotFoundByStatus(t *testing.T, env *testEnvironment, txID uuid.UUID, status wallet.TransactionStatus) {
	statusStr := status.String()
	txns, err := env.processingClient.GetTransactions(&api.GetTransactionsFilter{
		Status: statusStr,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, filteredTx := range txns {
		if filteredTx.ID == txID {
			t.Error("GetTransactions API returns tx with status %s "+
				"when it should not", statusStr)
		}
	}
}

func testWithdrawMultiple(t *testing.T, env *testEnvironment) {
	// TODO
}
