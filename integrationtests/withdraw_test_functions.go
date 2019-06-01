// +build integration

package integrationtests

import (
	"context"
	"testing"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
)

func testWithdraw(t *testing.T, env *testenv.TestEnvironment) {
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)
	clientBalance := getStableClientBalanceOrFail(t, env)

	runSubtest(t, "WithoutManualConfirmation", func(t *testing.T) {
		expectedClientBalanceAfterWithdraw := clientBalance + withdrawAmountSmall - withdrawFee

		tx := testMakeWithdraw(t, env, withdrawAddress, withdrawAmountSmall, nil)

		runSubtest(t, "NewTransaction", func(t *testing.T) {
			testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})

		wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")) // deposit - withdraw: 0.5 - 0.05
		checkBalance(t, env, wantBalance, wantBalance)

		tx.mineOrFail(t, env)

		runSubtest(t, "ConfirmedTransaction", func(t *testing.T) {
			testWithdrawFullyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
		})
	})

	clientBalance = getStableClientBalanceOrFail(t, env)

	runSubtest(t, "WithManualConfirmation", func(t *testing.T) {
		withdrawAmountBig := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.15"))

		tx := testMakeWithdraw(t, env, withdrawAddress, withdrawAmountBig, nil)

		runSubtest(t, "NewTransactionNotConfirmedYet", func(t *testing.T) {
			testWithdrawTransactionPendingManualConfirmation(t, env, tx,
				bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")), true)
		})
		expectedClientBalanceAfterWithdraw := clientBalance + withdrawAmountBig - withdrawFee
		runSubtest(t, "ManuallyConfirmTransaction", func(t *testing.T) {
			err := env.ProcessingClient.Confirm(tx.id)

			if err != nil {
				t.Fatalf("Failed to confirm tx: %v", err)
			}
			testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})

		// deposit - small withdraw - big withdraw: 0.5 - 0.05 - 0.15
		wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.3"))
		checkBalance(t, env, wantBalance, wantBalance)

		tx.mineOrFail(t, env)

		runSubtest(t, "ConfirmedTransaction", func(t *testing.T) {
			testWithdrawFullyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
		})

		runSubtest(t, "CancelInsteadOfConfirming", func(t *testing.T) {
			tx := testMakeWithdraw(t, env, withdrawAddress, withdrawAmountBig, nil)
			testWithdrawTransactionPendingManualConfirmation(t, env, tx,
				wantBalance, true)
			err := env.ProcessingClient.Cancel(tx.id)

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
		withdrawID := uuid.Must(uuid.FromString("e06ed38b-ff2c-4e3d-885f-135fe6c72625"))
		req := &wallet.WithdrawRequest{
			ID:      withdrawID,
			Amount:  withdrawAmountSmall,
			Address: withdrawAddress,
			Fee:     withdrawFee,
		}
		resp, err := env.ProcessingClient.Withdraw(req)
		if err != nil {
			t.Fatal(err)
		}
		checkClientWithdrawRequest(t, resp, req)
		if resp.ID != withdrawID {
			t.Errorf("Expected resulting withdraw id to be equal to "+
				"requested one %s, but got %s", withdrawID, resp.ID)
		}
		notification := env.GetNextCallbackNotificationWithTimeout(t)
		if notification.ID != withdrawID {
			t.Errorf("Expected withdraw id in http notification to be equal "+
				"to requested one %s, but got %s", withdrawID, notification.ID)
		}
		event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
		data := event.Data.(*wallet.TxNotification)
		if data.ID != req.ID {
			t.Errorf("Expected withdraw id in http notification to be equal "+
				"to requested one %s, but got %s", withdrawID, data.ID)
		}
		env.MineTx(notification.Hash)
		// skip corresponding notifications
		env.GetNextCallbackNotificationWithTimeout(t)
		env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
	})
	runSubtest(t, "DuplicateFixedID", func(t *testing.T) {
		ourBalance := getStableBalanceOrFail(t, env)
		withdrawID := uuid.Must(uuid.FromString("e06ed38b-ff2c-4e3d-885f-135fe6c72625"))
		req := &wallet.WithdrawRequest{
			ID:      withdrawID,
			Amount:  withdrawAmountSmall,
			Address: withdrawAddress,
			Fee:     withdrawFee,
		}
		_, err := env.ProcessingClient.Withdraw(req)
		if err == nil {
			t.Fatal(
				"Expected duplicate tx id to cause withdraw error, but " +
					"it did not",
			)
		}

		// check that balances did not change
		checkBalance(t, env, ourBalance, ourBalance)
		testenv.GenerateBlocks(env.Regtest["node-miner"].NodeAPI, 1)
		checkBalance(t, env, ourBalance, ourBalance)
	})
}

func testMakeWithdraw(t *testing.T, env *testenv.TestEnvironment, address string, amount bitcoin.BTCAmount, metainfo interface{}) *txTestData {
	req := &wallet.WithdrawRequest{
		Address: address, Amount: amount, Fee: withdrawFee, Metainfo: metainfo,
	}
	resp, err := env.ProcessingClient.Withdraw(req)
	if err != nil {
		t.Fatal(err)
	}
	checkClientWithdrawRequest(t, resp, req)

	return &txTestData{
		id:       resp.ID,
		address:  address,
		amount:   amount,
		fee:      withdrawFee,
		metainfo: metainfo,
	}
}

func testWithdrawInsufficientFunds(t *testing.T, env *testenv.TestEnvironment) {
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

func testWithdrawInsufficientFundsPending(t *testing.T, env *testenv.TestEnvironment, cancel bool) {
	ourBalance := getStableBalanceOrFail(t, env)
	withdrawAmountTooBig := ourBalance + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))
	unconfirmedIncome := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("2"))

	clientAccount, err := env.ProcessingClient.NewWallet(nil)

	if err != nil {
		t.Fatal(err)
	}
	// skip corresponding notification
	env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
	unconfirmedIncomeTxHash, err := env.Regtest["node-client"].NodeAPI.SendWithPerKBFee(
		clientAccount.Address, unconfirmedIncome, depositFee, false,
	)

	// skip corresponding notifications
	env.GetNextCallbackNotificationWithTimeout(t)
	env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

	checkBalance(t, env, ourBalance, ourBalance+unconfirmedIncome)

	tx := testMakePendingWithdraw(t, env, withdrawAmountTooBig)

	clientBalance := getStableClientBalanceOrFail(t, env)

	runSubtest(t, "GetTransactionsPending", func(t *testing.T) {
		testGetTransactionsTxFoundByStatus(t, env, tx.id, wallet.PendingTransaction)
	})

	if cancel {
		err := env.ProcessingClient.Cancel(tx.id)
		if err != nil {
			t.Fatal(err)
		}
		testWithdrawTransactionCancelled(t, env, tx)
		checkBalance(t, env, ourBalance, ourBalance+unconfirmedIncome)
	}

	_, err = env.MineTx(unconfirmedIncomeTxHash)

	if err != nil {
		t.Fatal(err)
	}

	if cancel {
		// skip notifications from incoming tx confirmation
		env.GetNextCallbackNotificationWithTimeout(t)
		env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
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

	tx.mineOrFail(t, env)

	testWithdrawFullyConfirmed(t, env, tx, clientBalanceAfterWithdraw)
}

func testWithdrawInsufficientFundsPendingColdStorage(t *testing.T, env *testenv.TestEnvironment, cancel bool) {
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
		err := env.ProcessingClient.Cancel(tx.id)
		if err != nil {
			t.Fatal(err)
		}
		testWithdrawTransactionCancelled(t, env, tx)
		checkRequiredFromColdStorage(t, env, zeroBTC)
		checkBalance(t, env, ourBalance, ourBalance)
		return
	}

	hsAddress, err := env.ProcessingClient.GetHotStorageAddress()

	if err != nil {
		t.Fatal(err)
	}

	incomeAmount := overflowAmount + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))

	incomeTxHash, err := env.Regtest["node-client"].NodeAPI.SendWithPerKBFee(
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

	_, err = env.MineTx(incomeTxHash)

	if err != nil {
		t.Fatal(err)
	}

	clientBalance := getStableClientBalanceOrFail(t, env)
	clientBalanceAfterWithdraw := clientBalance + withdrawAmountTooBig - withdrawFee

	testWithdrawNewTransaction(t, env, tx, clientBalance, clientBalanceAfterWithdraw)

	tx.mineOrFail(t, env)

	testWithdrawFullyConfirmed(t, env, tx, clientBalanceAfterWithdraw)
}

func testMakePendingWithdraw(t *testing.T, env *testenv.TestEnvironment, amount bitcoin.BTCAmount) *txTestData {
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)

	tx := testMakeWithdraw(t, env, withdrawAddress, amount, nil)

	// first, this withdraw will wait for manual confirmation
	testWithdrawTransactionPendingManualConfirmation(t, env, tx, zeroBTC, false)

	err := env.ProcessingClient.Confirm(tx.id)

	if err != nil {
		t.Fatal(err)
	}

	// alright, now it should become pending
	testWithdrawTransactionPending(t, env, tx, zeroBTC, false)

	return tx
}

func testWithdrawNewTransaction(t *testing.T, env *testenv.TestEnvironment, tx *txTestData, clientBalance, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)
	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
	checkNewWithdrawTransactionNotificationAndEvent(t, env, notification,
		event, tx, clientBalance, expectedClientBalanceAfterWithdraw)
}

func testWithdrawPartiallyConfirmed(t *testing.T, env *testenv.TestEnvironment, tx *txTestData, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)
	checkNotificationFieldsForPartiallyConfirmedClientWithdraw(t, notification, tx)
	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

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

func testWithdrawFullyConfirmed(t *testing.T, env *testenv.TestEnvironment, tx *txTestData, expectedClientBalanceAfterWithdraw bitcoin.BTCAmount) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)
	checkNotificationFieldsForFullyConfirmedClientWithdraw(t, notification, tx)
	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

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

func testWithdrawTransactionPendingManualConfirmation(t *testing.T, env *testenv.TestEnvironment, tx *txTestData, ourOldBalance bitcoin.BTCAmount, testBalance bool) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForWithdrawPendingManualConfirmation(t, notification, tx)

	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

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

func testWithdrawTransactionPendingColdStorage(t *testing.T, env *testenv.TestEnvironment, tx *txTestData, ourOldBalance bitcoin.BTCAmount, testBalance bool) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForWithdrawPendingColdStorage(t, notification, tx)

	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

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

func testWithdrawTransactionPending(t *testing.T, env *testenv.TestEnvironment, tx *txTestData, ourOldBalance bitcoin.BTCAmount, testBalance bool) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForWithdrawPending(t, notification, tx)

	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

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

func testWithdrawTransactionCancelled(t *testing.T, env *testenv.TestEnvironment, tx *txTestData) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)

	checkNotificationFieldsForCancelledWithdrawal(t, notification, tx)

	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

	if got, want := event.Type, events.PendingTxCancelledEvent; got != want {
		t.Errorf("Expected event type to be %s, but got %s", want, got)
	}

	data := event.Data.(*wallet.TxNotification)

	checkNotificationFieldsForCancelledWithdrawal(t, data, tx)
}

func testWithdrawSeveralConfirmations(t *testing.T, env *testenv.TestEnvironment, neededConfirmations int) {
	clientBalance := getStableClientBalanceOrFail(t, env)
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)
	expectedClientBalanceAfterWithdraw := clientBalance + withdrawAmountSmall - withdrawFee

	tx := testMakeWithdraw(t, env, withdrawAddress, withdrawAmountSmall, nil)

	runSubtest(t, "NewTransaction", func(t *testing.T) {
		testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
	})

	wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")) // deposit - withdraw: 0.5 - 0.05
	checkBalance(t, env, wantBalance, wantBalance)

	tx.mineOrFail(t, env)

	runSubtest(t, "Confirmation", func(t *testing.T) {
		runSubtest(t, "First", func(t *testing.T) {
			testWithdrawPartiallyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
		})
		runSubtest(t, "Successive", func(t *testing.T) {
			for i := 2; i < neededConfirmations; i++ {
				_, err := testenv.GenerateBlocks(env.Regtest["node-miner"].NodeAPI, 1)
				if err != nil {
					t.Fatal(err)
				}
				tx.confirmations = int64(i)
				testWithdrawPartiallyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
			}
			_, err := testenv.GenerateBlocks(env.Regtest["node-miner"].NodeAPI, 1)
			if err != nil {
				t.Fatal(err)
			}
			tx.confirmations++

			testWithdrawFullyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
		})
	})
}

func testGetTransactionsTxFoundByStatus(t *testing.T, env *testenv.TestEnvironment, txID uuid.UUID, status wallet.TransactionStatus) {
	statusStr := status.String()
	txns, err := env.ProcessingClient.GetTransactions(&api.GetTransactionsFilter{
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

func testGetTransactionsTxNotFoundByStatus(t *testing.T, env *testenv.TestEnvironment, txID uuid.UUID, status wallet.TransactionStatus) {
	statusStr := status.String()
	txns, err := env.ProcessingClient.GetTransactions(&api.GetTransactionsFilter{
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

func testWithdrawMultiple(t *testing.T, env *testenv.TestEnvironment) {
	const nWithdrawals = 3

	var (
		amounts   []bitcoin.BTCAmount
		addresses []string
	)

	for i := 1; i <= nWithdrawals; i++ {
		amounts = append(amounts, bitcoin.BTCAmount(i)*bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.01")))
		addresses = append(addresses, getNewAddressForWithdrawOrFail(t, env))
	}

	tests := map[string]bool{"DifferentAddresses": true, "SameAddress": false}

	for testName, useDifferentAddresses := range tests {
		runSubtest(t, testName, func(t *testing.T) {
			runSubtest(t, "Simultaneous", func(t *testing.T) {
				testWithdrawMultipleSimultaneous(t, env, addresses, amounts, useDifferentAddresses)
			})
			runSubtest(t, "Interleaved", func(t *testing.T) {
				testWithdrawMultipleInterleaved(t, env, addresses, amounts, useDifferentAddresses)
			})
		})
	}
}

func testWithdrawMultipleSimultaneous(t *testing.T, env *testenv.TestEnvironment, addresses []string, amounts []bitcoin.BTCAmount, useDifferentAddresses bool) {
	balanceByNow := getStableBalanceOrFail(t, env)
	nWithdrawals := len(amounts)

	totalWithdrawAmount := zeroBTC

	for _, amount := range amounts {
		totalWithdrawAmount += amount
	}

	if totalWithdrawAmount > balanceByNow {
		t.Fatalf("Test assumes wallet has enough money for withdrawals, but "+
			"current balance is %s, and total withdraw is %s", balanceByNow,
			totalWithdrawAmount)
	}

	balanceAfterWithdraw := balanceByNow - totalWithdrawAmount

	withdrawals := make(testTxCollection, nWithdrawals)
	var address string
	for i := 0; i < nWithdrawals; i++ {
		if useDifferentAddresses {
			address = addresses[i]
		} else {
			address = addresses[0]
		}
		withdrawals[i] = testMakeWithdraw(t, env, address, amounts[i],
			initialTestMetainfo)
	}
	runSubtest(t, "NewTransactions", func(t *testing.T) {
		httpNotifications, wsNotifications := collectNotifications(t, env, events.NewOutgoingTxEvent, nWithdrawals)

		for _, tx := range withdrawals {
			n := findNotificationForTxOrFail(t, httpNotifications, tx)
			checkNotificationFieldsForNewClientWithdraw(t, n, tx)
			tx.hash = n.Hash
			wsN := findNotificationForTxOrFail(t, wsNotifications, tx)
			checkNotificationFieldsForNewClientWithdraw(t, wsN, tx)
		}
		checkBalance(t, env, balanceAfterWithdraw, balanceAfterWithdraw)
	})

	withdrawals.mineOrFail(t, env)

	runSubtest(t, "ConfirmedTxns", func(t *testing.T) {
		httpNotifications, wsNotifications := collectNotifications(t, env, events.OutgoingTxConfirmedEvent, nWithdrawals)

		for _, tx := range withdrawals {
			n := findNotificationForTxOrFail(t, httpNotifications, tx)
			checkNotificationFieldsForFullyConfirmedClientWithdraw(t, n, tx)
			wsN := findNotificationForTxOrFail(t, wsNotifications, tx)
			if n.ID != tx.id {
				t.Errorf("Expected that tx id for confirmed tx in http callback "+
					"data to match id of initial tx, but they are %s %s",
					n.ID, tx.id)
			}

			if wsN.ID != tx.id {
				t.Errorf("Expected that tx id for confirmed tx in websocket "+
					"notification will match one for initial tx, but they are %s %s",
					tx.id, wsN.ID)
			}
			checkNotificationFieldsForFullyConfirmedClientWithdraw(t, wsN, tx)
		}
		checkBalance(t, env, balanceAfterWithdraw, balanceAfterWithdraw)
	})
}

func testWithdrawMultipleInterleaved(t *testing.T, env *testenv.TestEnvironment, addresses []string, amounts []bitcoin.BTCAmount, useDifferentAddresses bool) {
	balanceByNow := getStableBalanceOrFail(t, env)
	nWithdrawals := len(amounts)

	totalWithdrawAmount := zeroBTC

	for _, amount := range amounts {
		totalWithdrawAmount += amount
	}

	if totalWithdrawAmount > balanceByNow {
		t.Fatalf("Test assumes wallet has enough money for withdrawals, but "+
			"current balance is %s, and total withdraw is %s", balanceByNow,
			totalWithdrawAmount)
	}

	balanceAfterWithdraw := balanceByNow - totalWithdrawAmount

	var txYounger, txOlder *txTestData

	for i := 0; i < nWithdrawals; i++ {
		if txOlder != nil {
			txOlder.mineOrFail(t, env)
		}
		var accountIdx int
		if useDifferentAddresses {
			accountIdx = i
		} else {
			accountIdx = 0
		}
		txYounger = testMakeWithdraw(t, env, addresses[accountIdx], amounts[i], initialTestMetainfo)

		nEvents := 1
		if txOlder != nil {
			nEvents = 2
		}
		cbNotifications, wsEvents := collectNotificationsAndEvents(t, env, nEvents)
		if txOlder != nil {
			notification := findNotificationForTxOrFail(t, cbNotifications, txOlder)
			checkNotificationFieldsForFullyConfirmedClientWithdraw(t, notification, txOlder)
			event := findEventWithTypeOrFail(t, wsEvents, events.OutgoingTxConfirmedEvent)
			checkNotificationFieldsForFullyConfirmedClientWithdraw(t, event.Data.(*wallet.TxNotification), txOlder)
		}

		notification := findNotificationForTxOrFail(t, cbNotifications, txYounger)
		txYounger.hash = notification.Hash
		checkNotificationFieldsForNewClientWithdraw(t, notification, txYounger)
		event := findEventWithTypeOrFail(t, wsEvents, events.NewOutgoingTxEvent)
		eventData := event.Data.(*wallet.TxNotification)
		if eventData.ID != txYounger.id {
			t.Errorf("Expected tx id from cb and ws notification to be "+
				"equal, but they are %s %s", txYounger.id, eventData.ID)
		}
		checkNotificationFieldsForNewClientWithdraw(t, eventData, txYounger)
		txOlder = txYounger
	}
	txOlder.mineOrFail(t, env)
	cbNotifications, wsEvents := collectNotificationsAndEvents(t, env, 1)
	notification := findNotificationForTxOrFail(t, cbNotifications, txOlder)
	checkNotificationFieldsForFullyConfirmedClientWithdraw(t, notification, txOlder)
	event := findEventWithTypeOrFail(t, wsEvents, events.OutgoingTxConfirmedEvent)
	checkNotificationFieldsForFullyConfirmedClientWithdraw(t, event.Data.(*wallet.TxNotification), txOlder)

	checkBalance(t, env, balanceAfterWithdraw, balanceAfterWithdraw)
}

func testWithdrawToColdStorage(t *testing.T, env *testenv.TestEnvironment, ctx context.Context) {
	err := env.StartColdStorage(ctx)

	if err != nil {
		t.Fatalf("Failed to start cold storage: %v", err)
	}

	defer env.StopColdStorage(ctx)

	csAddress := env.ColdStorageLoadAndGenerateAddress()

	runSubtest(t, "Successful", func(t *testing.T) {
		balance := getStableBalanceOrFail(t, env)
		withdrawAmount := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))

		if balance < withdrawAmount {
			t.Fatal("Expected that wallet balance will be >= 1 BTC by now")
		}
		withdrawRequest := &wallet.WithdrawRequest{
			Address: csAddress, Amount: withdrawAmount, Fee: withdrawFee,
		}
		resp, err := env.ProcessingClient.WithdrawToColdStorage(withdrawRequest)
		if err != nil {
			t.Fatalf("Failed to perform withdraw to cold storage: %v", err)
		}
		checkCSWithdrawRequest(t, resp, withdrawRequest, "")
		checkBalance(t, env, balance-withdrawAmount, balance-withdrawAmount)
		// now, withdraw tx should get to miner and cold storage

		// wait for miner to get this tx. We don't know the hash, but there
		// should be no other txns in our test bitcoin network at this moment so
		// just wait for miner to get any tx
		_, err = env.MineAnyTx()

		if err != nil {
			t.Fatalf("Mining cold storage withdraw tx failed: %v", err)
		}

		// now our cold storage should have received money. It can happen after
		// some delay (when its node receives new block), so wait a bit
		coldStorageIncome := withdrawAmount - withdrawFee
		checkBalanceBecame(t, func() (*api.BalanceInfo, error) {
			return env.GetNodeBalance(env.Regtest["node-cold-storage"].NodeAPI)
		}, coldStorageIncome, coldStorageIncome)
	})

	runSubtest(t, "InsufficientFunds", func(t *testing.T) {
		balance := getStableBalanceOrFail(t, env)
		withdrawAmountTooBig := balance + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))
		_, err := env.ProcessingClient.WithdrawToColdStorage(&wallet.WithdrawRequest{
			Address: csAddress, Amount: withdrawAmountTooBig, Fee: withdrawFee,
		})
		if err == nil {
			t.Fatal("Expected that withdrawing more money than we have in " +
				"wallet to cold storage will raise an error but it did not")
		}
	})

	lastSeq := env.WebsocketListeners[0].LastSeq
	runSubtest(t, "AddressFromConfig", func(t *testing.T) {
		// we need to change processing config for this test, so we'll have to
		// restart it
		env.WebsocketListeners[0].Stop()
		env.WebsocketListeners = nil

		processingContainerID := env.Processing.ID
		err := env.StopProcessing(ctx)
		env.WaitForContainerRemoval(ctx, processingContainerID)

		if err != nil {
			t.Fatalf("Failed to stop processing for restart: %v", err)
		}

		settings := env.ProcessingSettings
		settings.AdditionalWalletSettings = "cold_wallet_address: " + csAddress

		err = env.StartProcessing(ctx, settings)

		if err != nil {
			t.Fatalf("Failed to start processing: %v", err)
		}

		env.WaitForProcessing()

		balance := getStableBalanceOrFail(t, env)
		withdrawAmount := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))

		if balance < withdrawAmount {
			t.Fatal("Expected that wallet balance will be >= 1 BTC by now")
		}

		csBalance, err := env.GetNodeBalance(env.Regtest["node-cold-storage"].NodeAPI)

		if err != nil {
			t.Fatalf("Failed to request balance from cold storage %v", err)
		}

		if csBalance.Balance != csBalance.BalanceWithUnconf {
			t.Fatal("Expected cold storage balance to be stable by this momen")
		}

		withdrawRequest := &wallet.WithdrawRequest{
			Amount: withdrawAmount, Fee: withdrawFee,
		}
		resp, err := env.ProcessingClient.WithdrawToColdStorage(withdrawRequest)
		if err != nil {
			t.Fatalf("Failed to perform withdraw to cold storage: %v", err)
		}
		checkCSWithdrawRequest(t, resp, withdrawRequest, csAddress)
		checkBalance(t, env, balance-withdrawAmount, balance-withdrawAmount)
		// now, withdraw tx should get to miner and cold storage

		// wait for miner to get this tx. We don't know the hash, but there
		// should be no other txns in our test bitcoin network at this moment so
		// just wait for miner to get any tx
		_, err = env.MineAnyTx()

		if err != nil {
			t.Fatalf("Mining cold storage withdraw tx failed: %v", err)
		}

		// now our cold storage should have received money. It can happen after
		// some delay (when its node receives new block), so wait a bit
		coldStorageIncome := withdrawAmount - withdrawFee
		checkBalanceBecame(t, func() (*api.BalanceInfo, error) {
			return env.GetNodeBalance(env.Regtest["node-cold-storage"].NodeAPI)
		}, csBalance.Balance+coldStorageIncome, csBalance.Balance+coldStorageIncome)
	})

	// recreate stopped WS listener
	_, err = env.NewWebsocketListener(lastSeq + 1)
}
