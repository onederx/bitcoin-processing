// +build integration

package integrationtests

import (
	"context"
	"testing"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

const zeroBTC = bitcoin.BTCAmount(0)

var (
	testDepositAmount   = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.5"))
	depositFee          = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.004"))
	withdrawFee         = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0001"))
	withdrawAmountSmall = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.05"))
)

type testMetainfo struct {
	Testing string `json:"testing"`
	Index   int    `json:"index"`
	Data    struct {
		User string `json:"user"`
	} `json:"data"`
}

var initialTestMetainfo = testMetainfo{
	Testing: "testtest",
	Index:   123,
	Data: struct {
		User string `json:"user"`
	}{User: "tester"},
}

func TestSmoke(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnvironment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stop(ctx)
	env.waitForLoad()
	err = env.startProcessingWithDefaultSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stopProcessing(ctx)
	env.waitForProcessing()

	_, err = env.processingClient.GetEvents(0)

	if err != nil {
		t.Fatal(err)
	}
}

func TestCommonUsage(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnvironment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stop(ctx)
	env.waitForLoad()
	err = env.startProcessingWithDefaultSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stopProcessing(ctx)
	env.waitForProcessing()
	_, err = env.newWebsocketListener(0)
	if err != nil {
		t.Fatalf("Failed to connect websocket event listener %v", err)
	}
	runSubtest(t, "HotWalletGenerated", func(t *testing.T) {
		testHotWalletGenerated(t, env)
	})
	runSubtest(t, "InitialCheckBalanceGivesZero", func(t *testing.T) {
		checkBalance(t, env, zeroBTC, zeroBTC)
	})
	var clientAccount *wallet.Account
	runSubtest(t, "GenerateClientWallet", func(t *testing.T) {
		clientAccount = testGenerateClientWallet(t, env)
	})
	runSubtest(t, "Deposit", func(t *testing.T) {
		testDeposit(t, env, clientAccount.Address)
	})
	runSubtest(t, "GetBalanceAfterDeposit", func(t *testing.T) {
		checkBalance(t, env, testDepositAmount, testDepositAmount)
	})
	runSubtest(t, "Withdraw", func(t *testing.T) {
		testWithdraw(t, env)
	})
	var accounts []*wallet.Account
	runSubtest(t, "MultipleAccounts", func(t *testing.T) {
		accounts = testGenerateMultipleClientWallets(t, env)
	})
	runSubtest(t, "MultipleDeposits", func(t *testing.T) {
		testDepositMultiple(t, env, accounts)
	})
	runSubtest(t, "MultipleWithdrawals", func(t *testing.T) {
		testWithdrawMultiple(t, env)
	})
	runSubtest(t, "MultipleDepositsAndWithdrawalsMixed", func(t *testing.T) {
		testDepositAndWithdrawMultipleMixed(t, env, accounts)
	})
}

func TestMoreConfirmations(t *testing.T) {
	const neededConfirmations = 4

	ctx := context.Background()
	env, err := newTestEnvironment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stop(ctx)
	env.waitForLoad()

	processingSettings := defaultSettings

	processingSettings.MaxConfirmations = neededConfirmations
	processingSettings.CallbackURL = env.callbackURL

	err = env.startProcessing(ctx, &processingSettings)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stopProcessing(ctx)
	env.waitForProcessing()
	_, err = env.newWebsocketListener(0)
	if err != nil {
		t.Fatalf("Failed to connect websocket event listener %v", err)
	}

	clientWallet, err := env.processingClient.NewWallet(nil)

	// skip new wallet notification
	env.websocketListeners[0].getNextMessageWithTimeout(t)

	runSubtest(t, "Deposit", func(t *testing.T) {
		testDepositSeveralConfirmations(t, env, clientWallet.Address, neededConfirmations)
	})
	runSubtest(t, "GetBalanceAfterDeposit", func(t *testing.T) {
		checkBalance(t, env, testDepositAmount, testDepositAmount)
	})
	runSubtest(t, "Withdraw", func(t *testing.T) {
		testWithdrawSeveralConfirmations(t, env, neededConfirmations)
	})
}

func testHotWalletGenerated(t *testing.T, env *testEnvironment) {
	hotWalletAddress, err := env.processingClient.GetHotStorageAddress()
	if err != nil {
		t.Fatalf("Failed to request hot wallet address %v", err)
	}
	if hotWalletAddress == "" {
		t.Fatalf("Hot wallet address from get_hot_storage_address API is empty")
	}
}

func testGenerateClientWallet(t *testing.T, env *testEnvironment) *wallet.Account {
	var clientAccount *wallet.Account
	runSubtest(t, "EmptyMetainfo", func(t *testing.T) {
		testGenerateClientWalletWithMetainfo(t, env, nil, 1)
	})
	runSubtest(t, "NonEmptyMetainfo", func(t *testing.T) {
		clientAccount = testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, 2)
	})
	return clientAccount
}

func testGenerateClientWalletWithMetainfo(t *testing.T, env *testEnvironment, metainfo interface{}, seq int) *wallet.Account {
	result, err := env.processingClient.NewWallet(metainfo)
	if err != nil {
		t.Fatal(err)
	}
	if metainfo != nil {
		if result.Metainfo == nil {
			t.Fatalf("Metainfo unexpectedly nil")
		} else {
			compareMetainfo(t, result.Metainfo, metainfo)
		}
	} else if metainfo == nil && result.Metainfo != nil {
		t.Errorf("Metainfo unexpectedly non-nil: %v", result.Metainfo)
	}
	address := result.Address
	if address == "" {
		t.Errorf("Generated address is empty")
	}
	event := env.websocketListeners[0].getNextMessageWithTimeout(t)
	if seq >= 0 {
		if got, want := event.Seq, seq; got != want {
			t.Errorf("Unxpected sequence number of second event: %d, wanted %d", got, want)
		}
	}
	if got, want := event.Type, events.NewAddressEvent; got != want {
		t.Errorf("Unexpected event type for new wallet generation, wanted %s, got %s:",
			want, got)
	}
	data := event.Data.(*wallet.Account)
	if got, want := data.Address, address; got != want {
		t.Errorf("Expected address from WS notification to be equal "+
			"to address from API response (%s), but instead got %s",
			want, got)
	}
	if metainfo != nil {
		compareMetainfo(t, data.Metainfo, metainfo)
	}
	return result
}

func testGenerateMultipleClientWallets(t *testing.T, env *testEnvironment) []*wallet.Account {
	type testData struct {
		ID int `json:"id"`
	}
	var result []*wallet.Account
	testMetainfos := []interface{}{
		struct {
			TestTest int        `json:"testtest"`
			Name     string     `json:"name"`
			Active   bool       `json:"active"`
			Info     []testData `json:"info"`
		}{
			TestTest: 321,
			Name:     "Foo Bar",
			Info:     []testData{testData{ID: 777}, testData{ID: 99}},
		},
		nil,
		initialTestMetainfo,
	}
	for _, m := range testMetainfos {
		result = append(result, testGenerateClientWalletWithMetainfo(t, env, m, -1))
	}
	return result
}

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
		// TODO
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
	balanceInfo, err := env.processingClient.GetBalance()

	if err != nil {
		t.Fatal(err)
	}

	if balanceInfo.Balance != balanceInfo.BalanceWithUnconf {
		t.Fatalf("Expected that confirmed and uncofirmed balance to be equal "+
			"by this moment, but they are %s %s", balanceInfo.Balance,
			balanceInfo.BalanceWithUnconf)
	}

	balanceByNow := balanceInfo.Balance

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
	balanceInfo, err := env.processingClient.GetBalance()

	if err != nil {
		t.Fatal(err)
	}

	if balanceInfo.Balance != balanceInfo.BalanceWithUnconf {
		t.Fatalf("Expected that confirmed and uncofirmed balance to be equal "+
			"by this moment, but they are %s %s", balanceInfo.Balance,
			balanceInfo.BalanceWithUnconf)
	}

	balanceByNow := balanceInfo.Balance

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

func testWithdrawMultiple(t *testing.T, env *testEnvironment) {
	// TODO
}

func testDepositAndWithdrawMultipleMixed(t *testing.T, env *testEnvironment, accounts []*wallet.Account) {
	// TODO
}
