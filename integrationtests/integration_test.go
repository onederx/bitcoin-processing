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
	t.Run("HotWalletGenerated", func(t *testing.T) {
		testHotWalletGenerated(t, env)
	})
	t.Run("InitialCheckBalanceGivesZero", func(t *testing.T) {
		checkBalance(t, env, zeroBTC, zeroBTC)
	})
	var clientAddress string
	t.Run("GenerateClientWallet", func(t *testing.T) {
		clientAddress = testGenerateClientWallet(t, env)
	})
	t.Run("Deposit", func(t *testing.T) {
		testDeposit(t, env, clientAddress)
	})
	t.Run("GetBalanceAfterDeposit", func(t *testing.T) {
		checkBalance(t, env, testDepositAmount, testDepositAmount)
	})
	t.Run("Withdraw", func(t *testing.T) {
		testWithdraw(t, env)
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

	t.Run("Deposit", func(t *testing.T) {
		testDepositSeveralConfirmations(t, env, clientWallet.Address, neededConfirmations)
	})
	t.Run("GetBalanceAfterDeposit", func(t *testing.T) {
		checkBalance(t, env, testDepositAmount, testDepositAmount)
	})
	t.Run("Withdraw", func(t *testing.T) {
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

func testGenerateClientWallet(t *testing.T, env *testEnvironment) string {
	var clientAddress string
	t.Run("EmptyMetainfo", func(t *testing.T) {
		result, err := env.processingClient.NewWallet(nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Metainfo != nil {
			t.Fatalf("Metainfo unexpectedly non-nil: %v", result.Metainfo)
		}
		address := result.Address
		if address == "" {
			t.Fatalf("Generated address is empty")
		}
		event := env.websocketListeners[0].getNextMessageWithTimeout(t)
		if got, want := event.Seq, 1; got != want {
			t.Errorf("Unxpected sequence number of first event: %d", got)
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
		if data.Metainfo != nil {
			t.Errorf("Account metainfo in WS notification unexpectedly non-nil: %v",
				data.Metainfo)
		}
	})
	t.Run("NonEmptyMetainfo", func(t *testing.T) {
		result, err := env.processingClient.NewWallet(initialTestMetainfo)
		if err != nil {
			t.Fatal(err)
		}
		if result.Metainfo == nil {
			t.Fatalf("Metainfo unexpectedly nil")
		}
		compareMetainfo(t, result.Metainfo, initialTestMetainfo)
		clientAddress = result.Address
		if clientAddress == "" {
			t.Fatalf("Generated address is empty")
		}
		event := env.websocketListeners[0].getNextMessageWithTimeout(t)
		if got, want := event.Seq, 2; got != want {
			t.Errorf("Unxpected sequence number of second event: %d", got)
		}
		if got, want := event.Type, events.NewAddressEvent; got != want {
			t.Errorf("Unexpected event type for new wallet generation, wanted %s, got %s:",
				want, got)
		}
		data := event.Data.(*wallet.Account)
		if got, want := data.Address, clientAddress; got != want {
			t.Errorf("Expected address from WS notification to be equal "+
				"to address from API response (%s), but instead got %s",
				want, got)
		}
		compareMetainfo(t, data.Metainfo, initialTestMetainfo)
	})
	return clientAddress
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

	t.Run("NewTransaction", func(t *testing.T) {
		req := env.getNextCallbackRequestWithTimeout(t)

		t.Run("CallbackMethodAndUrl", func(t *testing.T) {
			if got, want := req.method, "POST"; got != want {
				t.Errorf("Expected callback request to use method %s, instead was %s", want, got)
			}
			if got, want := req.url.Path, defaultCallbackURLPath; got != want {
				t.Errorf("Callback path should be %s, instead got %s", want, got)
			}
		})

		t.Run("CallbackNewDepositData", func(t *testing.T) {
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

	t.Run("ConfirmedTransaction", func(t *testing.T) {
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

	t.Run("WithoutManualConfirmation", func(t *testing.T) {
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

		t.Run("NewTransaction", func(t *testing.T) {
			testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})

		wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")) // deposit - withdraw: 0.5 - 0.05
		checkBalance(t, env, wantBalance, wantBalance)

		tx.blockHash, err = env.mineTx(tx.hash)

		if err != nil {
			t.Fatalf("Failed to mine tx into blockchain: %v", err)
		}

		tx.confirmations = 1

		t.Run("ConfirmedTransaction", func(t *testing.T) {
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

	t.Run("WithManualConfirmation", func(t *testing.T) {
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

		t.Run("NewTransactionNotConfirmedYet", func(t *testing.T) {
			testWithdrawTransactionPendingManualConfirmation(t, env, tx,
				bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")))
		})
		expectedClientBalanceAfterWithdraw := clientBalance.Balance + withdrawAmountBig - withdrawFee
		t.Run("ManuallyConfirmTransaction", func(t *testing.T) {
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

		t.Run("ConfirmedTransaction", func(t *testing.T) {
			testWithdrawFullyConfirmed(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
		})

		t.Run("CancelInsteadOfConfirming", func(t *testing.T) {
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

	t.Run("FixedID", func(t *testing.T) {
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

	t.Run("NewTransaction", func(t *testing.T) {
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

	t.Run("Confirmation", func(t *testing.T) {
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

	t.Run("NewTransaction", func(t *testing.T) {
		testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
	})

	wantBalance := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.45")) // deposit - withdraw: 0.5 - 0.05
	checkBalance(t, env, wantBalance, wantBalance)

	tx.blockHash, err = env.mineTx(tx.hash)

	if err != nil {
		t.Fatalf("Failed to mine tx into blockchain: %v", err)
	}

	tx.confirmations = 1

	t.Run("Confirmation", func(t *testing.T) {
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
