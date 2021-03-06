// +build integration

package integrationtests

import (
	"fmt"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
	wallettypes "github.com/onederx/bitcoin-processing/wallet/types"
)

func testDeposit(t *testing.T, env *testenv.TestEnvironment, clientAddress string) {
	tx := testMakeDeposit(t, env, clientAddress, testDepositAmount, initialTestMetainfo)
	runSubtest(t, "NewTransaction", func(t *testing.T) {
		req := env.GetNextCallbackRequestWithTimeout(t)

		runSubtest(t, "CallbackMethodAndUrl", func(t *testing.T) {
			if got, want := req.Method, "POST"; got != want {
				t.Errorf("Expected callback request to use method %s, instead was %s", want, got)
			}
			if got, want := req.URL.Path, testenv.DefaultCallbackURLPath; got != want {
				t.Errorf("Callback path should be %s, instead got %s", want, got)
			}
		})

		runSubtest(t, "CallbackNewDepositData", func(t *testing.T) {
			notification := req.UnmarshalOrFail(t)

			tx.id = notification.ID
			checkNotificationFieldsForNewDeposit(t, notification, tx)
		})

		event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
		data := event.Data.(*wallettypes.TxNotification)
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

	tx.mineOrFail(t, env)

	runSubtest(t, "ConfirmedTransaction", func(t *testing.T) {
		testDepositFullyConfirmed(t, env, tx)
	})
}

func testMakeDeposit(t *testing.T, env *testenv.TestEnvironment, address string, amount bitcoin.BTCAmount, metainfo interface{}) *txTestData {
	txHash, err := env.Regtest["node-client"].NodeAPI.SendWithPerKBFee(
		address, amount, depositFee, false,
	)

	if err != nil {
		t.Fatalf("Failed to send money from client node for deposit")
	}

	return &txTestData{
		address:  address,
		amount:   amount,
		hash:     txHash,
		metainfo: metainfo,
	}
}

func testDepositPartiallyConfirmed(t *testing.T, env *testenv.TestEnvironment, tx *txTestData) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)

	if notification.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in http callback "+
			"data to match id of initial tx, but they are %s %s",
			notification.ID, tx.id)
	}
	checkNotificationFieldsForPartiallyConfirmedDeposit(t, notification, tx)

	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
	if got, want := event.Type, events.IncomingTxConfirmedEvent; got != want {
		t.Errorf("Unexpected event type for confirmed deposit, wanted %s, got %s:",
			want, got)
	}
	data := event.Data.(*wallettypes.TxNotification)
	if data.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in websocket "+
			"notification will match one for initial tx, but they are %s %s",
			tx.id, data.ID)
	}
	checkNotificationFieldsForPartiallyConfirmedDeposit(t, data, tx)

}

func testDepositFullyConfirmed(t *testing.T, env *testenv.TestEnvironment, tx *txTestData) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)

	if notification.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in http callback "+
			"data to match id of initial tx, but they are %s %s",
			notification.ID, tx.id)
	}
	checkNotificationFieldsForFullyConfirmedDeposit(t, notification, tx)

	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
	if got, want := event.Type, events.IncomingTxConfirmedEvent; got != want {
		t.Errorf("Unexpected event type for confirmed deposit, wanted %s, got %s:",
			want, got)
	}
	data := event.Data.(*wallettypes.TxNotification)
	if data.ID != tx.id {
		t.Errorf("Expected that tx id for confirmed tx in websocket "+
			"notification will match one for initial tx, but they are %s %s",
			tx.id, data.ID)
	}
	checkNotificationFieldsForFullyConfirmedDeposit(t, data, tx)
}

func testDepositSeveralConfirmations(t *testing.T, env *testenv.TestEnvironment, clientAddress string, neededConfirmations int) {
	tx := testMakeDeposit(t, env, clientAddress, testDepositAmount, nil)

	runSubtest(t, "NewTransaction", func(t *testing.T) {
		notification := env.GetNextCallbackNotificationWithTimeout(t)
		tx.id = notification.ID
		checkNotificationFieldsForNewDeposit(t, notification, tx)

		event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
		data := event.Data.(*wallettypes.TxNotification)
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

	tx.mineOrFail(t, env)

	runSubtest(t, "Confirmation", func(t *testing.T) {
		runSubtest(t, "First", func(t *testing.T) {
			testDepositPartiallyConfirmed(t, env, tx)
		})
		runSubtest(t, "Successive", func(t *testing.T) {
			for i := 2; i < neededConfirmations; i++ {
				_, err := testenv.GenerateBlocks(env.Regtest["node-miner"].NodeAPI, 1)
				if err != nil {
					t.Fatal(err)
				}
				tx.confirmations = int64(i)
				testDepositPartiallyConfirmed(t, env, tx)
			}
			_, err := testenv.GenerateBlocks(env.Regtest["node-miner"].NodeAPI, 1)
			if err != nil {
				t.Fatal(err)
			}
			tx.confirmations++
			testDepositFullyConfirmed(t, env, tx)
		})
	})
}

func testDepositMultiple(t *testing.T, env *testenv.TestEnvironment, accounts []*wallet.Account) {
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

func testDepositMultipleSimultaneous(t *testing.T, env *testenv.TestEnvironment, accounts []*wallet.Account, amounts []bitcoin.BTCAmount, useDifferentAddresses bool, nDeposits int) {
	balanceByNow := getStableBalanceOrFail(t, env)

	// 0.1 + 0.2 + 0.3 = 0.6
	balanceAfterDeposit := balanceByNow + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.6"))

	runSubtest(t, "Simultaneous", func(t *testing.T) {
		deposits := make(testTxCollection, nDeposits)
		for i := 0; i < nDeposits; i++ {
			var account *wallet.Account
			if useDifferentAddresses {
				account = accounts[i]
			} else {
				account = accounts[0]
			}

			deposits[i] = testMakeDeposit(t, env, account.Address, amounts[i], account.Metainfo)
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

		deposits.mineOrFail(t, env)

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

func testDepositMultipleInterleaved(t *testing.T, env *testenv.TestEnvironment, accounts []*wallet.Account, amounts []bitcoin.BTCAmount, useDifferentAddresses bool, nDeposits int) {
	balanceByNow := getStableBalanceOrFail(t, env)

	var txYounger, txOlder *txTestData

	runSubtest(t, "Interleaved", func(t *testing.T) {
		for i := 0; i < nDeposits; i++ {
			if txOlder != nil {
				txOlder.mineOrFail(t, env)
			}
			var accountIdx int
			if useDifferentAddresses {
				accountIdx = i
			} else {
				accountIdx = 0
			}

			txYounger = testMakeDeposit(t, env, accounts[accountIdx].Address, amounts[i], accounts[accountIdx].Metainfo)

			nEvents := 1
			if txOlder != nil {
				nEvents = 2
			}
			cbNotifications, wsEvents := collectNotificationsAndEvents(t, env, nEvents)
			if txOlder != nil {
				notification := findNotificationForTxOrFail(t, cbNotifications, txOlder)
				checkNotificationFieldsForFullyConfirmedDeposit(t, notification, txOlder)
				event := findEventWithTypeOrFail(t, wsEvents, events.IncomingTxConfirmedEvent)
				checkNotificationFieldsForFullyConfirmedDeposit(t, event.Data.(*wallettypes.TxNotification), txOlder)
			}

			notification := findNotificationForTxOrFail(t, cbNotifications, txYounger)
			txYounger.id = notification.ID
			checkNotificationFieldsForNewDeposit(t, notification, txYounger)
			event := findEventWithTypeOrFail(t, wsEvents, events.NewIncomingTxEvent)
			eventData := event.Data.(*wallettypes.TxNotification)
			if eventData.ID != txYounger.id {
				t.Errorf("Expected tx id from cb and ws notification to be "+
					"equal, but they are %s %s", txYounger.id, eventData.ID)
			}
			checkNotificationFieldsForNewDeposit(t, eventData, txYounger)
			txOlder = txYounger
		}
		txOlder.mineOrFail(t, env)
		cbNotifications, wsEvents := collectNotificationsAndEvents(t, env, 1)
		notification := findNotificationForTxOrFail(t, cbNotifications, txOlder)
		checkNotificationFieldsForFullyConfirmedDeposit(t, notification, txOlder)
		event := findEventWithTypeOrFail(t, wsEvents, events.IncomingTxConfirmedEvent)
		checkNotificationFieldsForFullyConfirmedDeposit(t, event.Data.(*wallettypes.TxNotification), txOlder)

		wantBalance := balanceByNow + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.6"))
		checkBalance(t, env, wantBalance, wantBalance)
	})
}

func testSendFundsToHotWallet(t *testing.T, env *testenv.TestEnvironment, hotWalletAddress string) {
	balance := getStableBalanceOrFail(t, env)
	hotWalletIncome := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.3"))
	expectedBalanceAfterIncome := balance + hotWalletIncome

	txHash, err := env.Regtest["node-client"].NodeAPI.SendWithPerKBFee(
		hotWalletAddress, hotWalletIncome, depositFee, false,
	)
	waitForEventOrFailTest(t, func() error {
		balanceInfo, err := env.ProcessingClient.GetBalance()

		if err != nil {
			return err
		}

		if balanceInfo.BalanceWithUnconf == expectedBalanceAfterIncome {
			return nil
		}
		return fmt.Errorf("Expected unconfirmed wallet balance to become %s "+
			"after tx sending money to hot wallet address was created, but "+
			"it is %s", expectedBalanceAfterIncome, balanceInfo.BalanceWithUnconf)
	})
	_, err = env.MineTx(txHash)
	if err != nil {
		t.Fatalf("Failed to mine tx %s into blockchain: %v", txHash, err)
	}
	waitForEventOrFailTest(t, func() error {
		balanceInfo, err := env.ProcessingClient.GetBalance()

		if err != nil {
			return err
		}

		if balanceInfo.Balance == expectedBalanceAfterIncome {
			return nil
		}
		return fmt.Errorf("Expected confirmed wallet balance to become %s "+
			"after tx sending money to hot wallet address was mined, but "+
			"it is %s", expectedBalanceAfterIncome, balanceInfo.Balance)
	})
}

func testDepositSingleBitcoinTxWithMultipleExists(t *testing.T, env *testenv.TestEnvironment, accounts []*wallet.Account) {
	balanceByNow := getStableBalanceOrFail(t, env)

	nDeposits := len(accounts)
	amounts := make(map[string]bitcoin.BTCAmount)

	// make up amounts we will send to them (just 0.1, 0.2, 0.3)
	for i, account := range accounts {
		amounts[account.Address] = bitcoin.BTCAmountFromFloat(0.1 * float64(i+1))
	}

	// 0.1 + 0.2 + 0.3
	balanceAfterDeposit := balanceByNow + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.6"))

	hash, err := env.Regtest["node-client"].NodeAPI.SendToMultipleAddresses(amounts)

	if err != nil {
		t.Fatalf("Failed to send money from client node for deposit: %v", err)
	}

	deposits := make(testTxCollection, nDeposits)

	for i := 0; i < nDeposits; i++ {
		deposits[i] = &txTestData{
			address:  accounts[i].Address,
			amount:   amounts[accounts[i].Address],
			hash:     hash,
			metainfo: accounts[i].Metainfo,
		}
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

	blockHash, err := env.MineTx(hash)

	if err != nil {
		t.Fatalf("Failed to mine deposit bitcoin tx on miner node: %v", err)
	}

	for i := 0; i < nDeposits; i++ {
		deposits[i].blockHash = blockHash
		deposits[i].confirmations = 1
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
}
