package integrationtests

import (
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

func testDeposit(t *testing.T, env *testEnvironment, clientAddress string) {
	txHash, err := env.regtest["node-client"].nodeAPI.SendWithPerKBFee(
		clientAddress, testDepositAmount, depositFee, false,
	)

	if err != nil {
		t.Fatalf("Failed to send money from client node for deposit")
	}

	tx := &txTestData{
		address:  clientAddress,
		amount:   testDepositAmount,
		hash:     txHash,
		metainfo: initialTestMetainfo,
	}

	runSubtest(t, "NewTransaction", func(t *testing.T) {
		req := env.getNextCallbackRequestWithTimeout(t)

		runSubtest(t, "CallbackMethodAndUrl", func(t *testing.T) {
			if got, want := req.method, "POST"; got != want {
				t.Errorf("Expected callback request to use method %s, instead was %s", want, got)
			}
			if got, want := req.url.Path, defaultCallbackURLPath; got != want {
				t.Errorf("Callback path should be %s, instead got %s", want, got)
			}
		})

		runSubtest(t, "CallbackNewDepositData", func(t *testing.T) {
			notification := req.unmarshalOrFail(t)

			tx.id = notification.ID
			checkNotificationFieldsForNewDeposit(t, notification, tx)
		})

		event := env.websocketListeners[0].getNextMessageWithTimeout(t)
		data := event.Data.(*wallet.TxNotification)
		if got, want := event.Type, events.NewIncomingTxEvent; got != want {
			t.Errorf("Unexpected event type for new deposit, wanted %s, got %s:",
				want, got)
		}
		if data.ID != tx.id {
			t.Errorf("Expected that tx id in websocket and http callback "+
				"notification will be the same, but they are %s %s",
				tx.id, data.ID)
		}
		checkNotificationFieldsForNewDeposit(t, data, tx)

		checkBalance(t, env, zeroBTC, testDepositAmount)
	})

	blockHash, err := env.mineTx(txHash)

	if err != nil {
		t.Fatalf("Failed to mine tx into blockchain: %v", err)
	}

	tx.confirmations = 1
	tx.blockHash = blockHash

	runSubtest(t, "ConfirmedTransaction", func(t *testing.T) {
		testDepositFullyConfirmed(t, env, tx)
	})
}

func testDepositPartiallyConfirmed(t *testing.T, env *testEnvironment, tx *txTestData) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	if notification.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in http callback "+
			"data to match id of initial tx, but they are %s %s",
			notification.ID, tx.id)
	}
	checkNotificationFieldsForPartiallyConfirmedDeposit(t, notification, tx)

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)
	if got, want := event.Type, events.IncomingTxConfirmedEvent; got != want {
		t.Errorf("Unexpected event type for confirmed deposit, wanted %s, got %s:",
			want, got)
	}
	data := event.Data.(*wallet.TxNotification)
	if data.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in websocket "+
			"notification will match one for initial tx, but they are %s %s",
			tx.id, data.ID)
	}
	checkNotificationFieldsForPartiallyConfirmedDeposit(t, data, tx)

}

func testDepositFullyConfirmed(t *testing.T, env *testEnvironment, tx *txTestData) {
	notification := env.getNextCallbackNotificationWithTimeout(t)

	if notification.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in http callback "+
			"data to match id of initial tx, but they are %s %s",
			notification.ID, tx.id)
	}
	checkNotificationFieldsForFullyConfirmedDeposit(t, notification, tx)

	event := env.websocketListeners[0].getNextMessageWithTimeout(t)
	if got, want := event.Type, events.IncomingTxConfirmedEvent; got != want {
		t.Errorf("Unexpected event type for confirmed deposit, wanted %s, got %s:",
			want, got)
	}
	data := event.Data.(*wallet.TxNotification)
	if data.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in websocket "+
			"notification will match one for initial tx, but they are %s %s",
			tx.id, data.ID)
	}
	checkNotificationFieldsForFullyConfirmedDeposit(t, data, tx)
}

func testDepositSeveralConfirmations(t *testing.T, env *testEnvironment, clientAddress string, neededConfirmations int) {
	txHash, err := env.regtest["node-client"].nodeAPI.SendWithPerKBFee(
		clientAddress, testDepositAmount, depositFee, false,
	)

	if err != nil {
		t.Fatalf("Failed to send money from client node for deposit")
	}

	tx := &txTestData{
		address:  clientAddress,
		amount:   testDepositAmount,
		hash:     txHash,
		metainfo: nil,
	}

	runSubtest(t, "NewTransaction", func(t *testing.T) {
		notification := env.getNextCallbackNotificationWithTimeout(t)
		tx.id = notification.ID
		checkNotificationFieldsForNewDeposit(t, notification, tx)

		event := env.websocketListeners[0].getNextMessageWithTimeout(t)
		data := event.Data.(*wallet.TxNotification)
		if got, want := event.Type, events.NewIncomingTxEvent; got != want {
			t.Errorf("Unexpected event type for new deposit, wanted %s, got %s:",
				want, got)
		}
		if data.ID != tx.id {
			t.Errorf("Expected that tx id in websocket and http callback "+
				"notification will be the same, but they are %s %s",
				tx.id, data.ID)
		}
		checkNotificationFieldsForNewDeposit(t, data, tx)

		checkBalance(t, env, zeroBTC, testDepositAmount)
	})

	blockHash, err := env.mineTx(txHash)

	if err != nil {
		t.Fatalf("Failed to mine tx into blockchain: %v", err)
	}

	tx.confirmations = 1
	tx.blockHash = blockHash

	runSubtest(t, "Confirmation", func(t *testing.T) {
		testDepositPartiallyConfirmed(t, env, tx)

		for i := 2; i < neededConfirmations; i++ {
			_, err := generateBlocks(env.regtest["node-miner"].nodeAPI, 1)
			if err != nil {
				t.Fatal(err)
			}
			tx.confirmations = int64(i)
			testDepositPartiallyConfirmed(t, env, tx)
		}
		_, err := generateBlocks(env.regtest["node-miner"].nodeAPI, 1)
		if err != nil {
			t.Fatal(err)
		}
		tx.confirmations++
		testDepositFullyConfirmed(t, env, tx)
	})
}

func testDepositMultiple(t *testing.T, env *testEnvironment, accounts []*wallet.Account) {
	const nDeposits = 3

	amounts := []bitcoin.BTCAmount{
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.1")),
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.2")),
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.3")),
	}

	tests := map[string]bool{"DifferentAddresses": true, "SameAddress": false}

	for testName, useDifferentAddresses := range tests {
		runSubtest(t, testName, func(t *testing.T) {
			testDepositMultipleSimultaneous(t, env, accounts, amounts, useDifferentAddresses, nDeposits)
			testDepositMultipleInterleaved(t, env, accounts, amounts, useDifferentAddresses, nDeposits)
		})
	}
}

func testDepositMultipleSimultaneous(t *testing.T, env *testEnvironment, accounts []*wallet.Account, amounts []bitcoin.BTCAmount, useDifferentAddresses bool, nDeposits int) {
	balanceByNow := getStableBalanceOrFail(t, env)

	// 0.1 + 0.2 + 0.3 = 0.6
	balanceAfterDeposit := balanceByNow + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.6"))

	var address string

	runSubtest(t, "Simultaneous", func(t *testing.T) {
		var deposits []*txTestData
		for i := 0; i < nDeposits; i++ {
			var metainfo interface{}
			if useDifferentAddresses {
				address = accounts[i].Address
				metainfo = accounts[i].Metainfo
			} else {
				address = accounts[0].Address
				metainfo = accounts[0].Metainfo
			}
			txHash, err := env.regtest["node-client"].nodeAPI.SendWithPerKBFee(
				address, amounts[i], depositFee, false,
			)
			if err != nil {
				t.Fatalf("Failed to send money from client node for deposit")
			}

			deposits = append(deposits, &txTestData{
				address:  address,
				amount:   amounts[i],
				hash:     txHash,
				metainfo: metainfo,
			})
		}

		runSubtest(t, "NewTransactions", func(t *testing.T) {
			httpNotifications, wsNotifications := collectNotifications(t, env, events.NewIncomingTxEvent, nDeposits)

			for _, tx := range deposits {
				n := findNotificationForTxOrFail(t, httpNotifications, tx)
				checkNotificationFieldsForNewDeposit(t, n, tx)
				tx.id = n.ID
				wsN := findNotificationForTxOrFail(t, wsNotifications, tx)
				checkNotificationFieldsForNewDeposit(t, wsN, tx)
			}
			checkBalance(t, env, balanceByNow, balanceAfterDeposit)
		})
		var txHashes []string
		for _, tx := range deposits {
			txHashes = append(txHashes, tx.hash)
		}
		blockHash, err := env.mineMultipleTxns(txHashes)

		if err != nil {
			t.Fatalf("Failed to mine tx into blockchain: %v", err)
		}

		for _, tx := range deposits {
			tx.confirmations = 1
			tx.blockHash = blockHash
		}
		runSubtest(t, "ConfirmedTxns", func(t *testing.T) {
			httpNotifications, wsNotifications := collectNotifications(t, env, events.IncomingTxConfirmedEvent, nDeposits)

			for _, tx := range deposits {
				n := findNotificationForTxOrFail(t, httpNotifications, tx)
				checkNotificationFieldsForFullyConfirmedDeposit(t, n, tx)
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
				checkNotificationFieldsForFullyConfirmedDeposit(t, wsN, tx)
			}
			checkBalance(t, env, balanceAfterDeposit, balanceAfterDeposit)
		})
	})
}

func testDepositMultipleInterleaved(t *testing.T, env *testEnvironment, accounts []*wallet.Account, amounts []bitcoin.BTCAmount, useDifferentAddresses bool, nDeposits int) {
	balanceByNow := getStableBalanceOrFail(t, env)

	var txYounger, txOlder *txTestData

	runSubtest(t, "Interleaved", func(t *testing.T) {
		for i := 0; i < nDeposits; i++ {
			if txOlder != nil {
				blockHash, err := env.mineTx(txOlder.hash)
				if err != nil {
					t.Fatal(err)
				}
				txOlder.blockHash = blockHash
				txOlder.confirmations = 1
			}
			var accountIdx int
			if useDifferentAddresses {
				accountIdx = i
			} else {
				accountIdx = 0
			}
			txYounger = &txTestData{
				address:  accounts[accountIdx].Address,
				amount:   amounts[i],
				metainfo: accounts[accountIdx].Metainfo,
			}
			txHash, err := env.regtest["node-client"].nodeAPI.SendWithPerKBFee(
				txYounger.address, txYounger.amount, depositFee, false,
			)
			txYounger.hash = txHash
			if err != nil {
				t.Fatalf("Failed to send money from client node for deposit")
			}
			nEvents := 1
			if txOlder != nil {
				nEvents = 2
			}
			cbNotifications, wsEvents := collectNotificationsAndEvents(t, env, nEvents)
			if txOlder != nil {
				notification := findNotificationForTxOrFail(t, cbNotifications, txOlder)
				checkNotificationFieldsForFullyConfirmedDeposit(t, notification, txOlder)
				event := findEventWithTypeOrFail(t, wsEvents, events.IncomingTxConfirmedEvent)
				checkNotificationFieldsForFullyConfirmedDeposit(t, event.Data.(*wallet.TxNotification), txOlder)
			}

			notification := findNotificationForTxOrFail(t, cbNotifications, txYounger)
			txYounger.id = notification.ID
			checkNotificationFieldsForNewDeposit(t, notification, txYounger)
			event := findEventWithTypeOrFail(t, wsEvents, events.NewIncomingTxEvent)
			eventData := event.Data.(*wallet.TxNotification)
			if eventData.ID != txYounger.id {
				t.Errorf("Expected tx id from cb and ws notification to be "+
					"equal, but they are %s %s", txYounger.id, eventData.ID)
			}
			checkNotificationFieldsForNewDeposit(t, eventData, txYounger)
			txOlder = txYounger
		}
		blockHash, err := env.mineTx(txOlder.hash)
		if err != nil {
			t.Fatal(err)
		}
		txOlder.blockHash = blockHash
		txOlder.confirmations = 1
		cbNotifications, wsEvents := collectNotificationsAndEvents(t, env, 1)
		notification := findNotificationForTxOrFail(t, cbNotifications, txOlder)
		checkNotificationFieldsForFullyConfirmedDeposit(t, notification, txOlder)
		event := findEventWithTypeOrFail(t, wsEvents, events.IncomingTxConfirmedEvent)
		checkNotificationFieldsForFullyConfirmedDeposit(t, event.Data.(*wallet.TxNotification), txOlder)

		wantBalance := balanceByNow + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.6"))
		checkBalance(t, env, wantBalance, wantBalance)
	})
}