// +build integration

package integrationtests

import (
	"context"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

var testDepositAmount = bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.5"))

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
		env.websocketListeners[0].checkNextMessage(func(event *events.NotificationWithSeq) {
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
		env.websocketListeners[0].checkNextMessage(func(event *events.NotificationWithSeq) {
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
	})
	return clientAddress
}

func testDeposit(t *testing.T, env *testEnvironment, clientAddress string) {
	depositFee := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.004"))

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
		env.checkNextCallbackRequest(func(req *callbackRequest) {
			t.Run("CallbackMethodAndUrl", func(t *testing.T) {
				if got, want := req.method, "POST"; got != want {
					t.Errorf("Expected callback request to use method %s, instead was %s", want, got)
				}
				if got, want := req.url.Path, defaultCallbackURLPath; got != want {
					t.Errorf("Callback path should be %s, instead got %s", want, got)
				}
			})
			t.Run("CallbackNewDepositData", func(t *testing.T) {
				notification, err := req.unmarshal()
				if err != nil {
					t.Fatalf("Failed to deserialize notification data from http "+
						"callback request body: %v", err)
				}
				tx.id = notification.ID
				checkNotificationFieldsForNewDeposit(t, notification, tx)
			})
		})

		env.websocketListeners[0].checkNextMessage(func(event *events.NotificationWithSeq) {
			data := event.Data.(*wallet.TxNotification)
			if data.ID != tx.id {
				t.Errorf("Expected that tx id in websocket and http callback "+
					"notification will be the same, but they are %s %s",
					tx.id, data.ID)
			}
			checkNotificationFieldsForNewDeposit(t, data, tx)
		})
	})

	blockHash, err := env.mineTx(txHash)

	if err != nil {
		t.Fatalf("Failed to mine tx into blockchain: %v", err)
	}

	tx.confirmations = 1
	tx.blockHash = blockHash

	t.Run("ConfirmedTransaction", func(t *testing.T) {
		env.checkNextCallbackRequest(func(req *callbackRequest) {
			notification, err := req.unmarshal()
			if err != nil {
				t.Fatalf("Failed to deserialize notification data from http "+
					"callback request body: %v", err)
			}
			if notification.ID != tx.id {
				t.Errorf("Expected that tx id for confirmed tx in http callback "+
					"data to match id of initial tx, but they are %s %s",
					notification.ID, tx.id)
			}
			checkNotificationFieldsForFullyConfirmedDeposit(t, notification, tx)
		})

		env.websocketListeners[0].checkNextMessage(func(event *events.NotificationWithSeq) {
			data := event.Data.(*wallet.TxNotification)
			if data.ID != tx.id {
				t.Errorf("Expected that tx id for confirmed tx in websocket "+
					"notification will match one for initial tx, but they are %s %s",
					tx.id, data.ID)
			}
			checkNotificationFieldsForFullyConfirmedDeposit(t, data, tx)
		})
	})
}

func testWithdraw(t *testing.T, env *testEnvironment) {
	//withdrawFee := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0001"))

	t.Run("WithoutManualConfirmation", func(t *testing.T) {
		//withdrawAmount := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.05"))
		//resp, err := env.processingClient.Withdraw()
	})
}
