// +build integration

package integrationtests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
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
	var clientAddress string
	t.Run("GenerateClientWallet", func(t *testing.T) {
		clientAddress = testGenerateClientWallet(t, env)
	})
	t.Run("Deposit", func(t *testing.T) {
		testDeposit(t, env, clientAddress)
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
		env.websocketListeners[0].checkNextMessage(func(message []byte) {
			var event newWalletEvent
			err := json.Unmarshal(message, &event)
			if err != nil {
				t.Fatalf("Failed to JSON-decode WS notification: %v", err)
			}
			if got, want := event.Seq, 1; got != want {
				t.Errorf("Unxpected sequence number of first event: %d", got)
			}
			if got, want := event.Type, events.NewAddressEvent; got != want {
				t.Errorf("Unexpected event type for new wallet generation, wanted %s, got %s:",
					want, got)
			}
			if got, want := event.Data.Address, address; got != want {
				t.Errorf("Expected address from WS notification to be equal "+
					"to address from API response (%s), but instead got %s",
					want, got)
			}
			if event.Data.Metainfo != nil {
				t.Errorf("Account metainfo in WS notification unexpectedly non-nil: %v",
					event.Data.Metainfo)
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
		env.websocketListeners[0].checkNextMessage(func(message []byte) {
			var event newWalletEvent
			err := json.Unmarshal(message, &event)
			if err != nil {
				t.Fatalf("Failed to JSON-decode WS notification: %v", err)
			}
			if got, want := event.Seq, 2; got != want {
				t.Errorf("Unxpected sequence number of second event: %d", got)
			}
			if got, want := event.Type, events.NewAddressEvent; got != want {
				t.Errorf("Unexpected event type for new wallet generation, wanted %s, got %s:",
					want, got)
			}
			if got, want := event.Data.Address, clientAddress; got != want {
				t.Errorf("Expected address from WS notification to be equal "+
					"to address from API response (%s), but instead got %s",
					want, got)
			}
			compareMetainfo(t, event.Data.Metainfo, initialTestMetainfo)
		})
	})
	return clientAddress
}

func testDeposit(t *testing.T, env *testEnvironment, clientAddress string) {
	depositAmount := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.5"))
	depositFee := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.004"))

	txHash, err := env.regtest["node-client"].nodeAPI.SendWithPerKBFee(
		clientAddress, depositAmount, depositFee, false,
	)

	if err != nil {
		t.Fatalf("Failed to send money from client node for deposit")
	}

	checkNotificationFieldsForDeposit := func(t *testing.T, n *wallet.TxNotification) {
		if got, want := n.Address, clientAddress; got != want {
			t.Errorf("Incorrect address for deposit: expected %s, got %s", want, got)
		}
		if got, want := n.Amount, depositAmount; got != want {
			t.Errorf("Incorrect amount for deposit: expected %s, got %s", want, got)
		}
		if got, want := n.Direction, wallet.IncomingDirection; got != want {
			t.Errorf("Incorrect direction for deposit: expected %s, got %s", want, got)
		}
		if got, want := n.Hash, txHash; got != want {
			t.Errorf("Incorrect tx hash for deposit: expected %s, got %s", want, got)
		}
		if got, want := n.IpnType, "deposit"; got != want {
			t.Errorf("Expected 'ipn_type' field to be '%s', instead got '%s'", want, got)
		}
		if got, want := n.Currency, "BTC"; got != want {
			t.Errorf("Currency should always be '%s', instead got '%s'", want, got)
		}
		txIDStr := n.ID.String()
		if txIDStr == "" || n.IpnID != txIDStr {
			t.Errorf("Expected tx id and 'ipn_id' field to be equal and nonempty "+
				"instead they were '%s' and '%s'", txIDStr, n.IpnID)
		}
		if n.ColdStorage != false {
			t.Errorf("Cold storage flag should be empty for any incoming tx, instead it was true")
		}
		compareMetainfo(t, n.Metainfo, initialTestMetainfo)
	}

	checkNotificationFieldsForNewDeposit := func(t *testing.T, n *wallet.TxNotification) {
		checkNotificationFieldsForDeposit(t, n)
		if got, want := n.BlockHash, ""; got != want {
			t.Errorf("Expected that block hash for new tx will be empty, instead got %s", got)
		}
		if got, want := n.Confirmations, int64(0); got != want {
			t.Errorf("Expected 0 confirmations for new tx, instead got %d", got)
		}
		if got, want := n.StatusCode, 0; got != want {
			t.Errorf("Expected status code 0 for new incoming tx, instead got %d", got)
		}
		if got, want := n.StatusStr, wallet.NewTransaction.String(); got != want {
			t.Errorf("Expected status name %s for new incoming tx, instead got %s", want, got)
		}
	}

	var txID uuid.UUID

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
				txID = notification.ID
				checkNotificationFieldsForNewDeposit(t, notification)
			})
		})

		env.websocketListeners[0].checkNextMessage(func(message []byte) {
			var event txEvent

			err := json.Unmarshal(message, &event)
			if err != nil {
				t.Fatalf("Failed to deserialize websocket notification about new deposit: %v", err)
			}
			if event.Data.ID != txID {
				t.Errorf("Expected that tx id in websocket and http callback "+
					"notification will be the same, but they are %s %s",
					txID, event.Data.ID)
			}
			checkNotificationFieldsForNewDeposit(t, event.Data)
		})
	})

	blockHash, err := env.mineTx(txHash)

	if err != nil {
		t.Fatalf("Failed to mine tx into blockchain: %v", err)
	}

	checkNotificationFieldsForConfirmedDeposit := func(t *testing.T, n *wallet.TxNotification) {
		checkNotificationFieldsForDeposit(t, n)
		if got, want := n.BlockHash, blockHash; got != want {
			t.Errorf("Expected that block hash will be %s, instead got %s", want, got)
		}
		if got, want := n.Confirmations, int64(1); got != want {
			t.Errorf("Expected %d confirmations for confirmed tx, instead got %d", want, got)
		}
		if got, want := n.StatusCode, 100; got != want {
			t.Errorf("Expected status code %d for confirmed tx, instead got %d", want, got)
		}
		if got, want := n.StatusStr, wallet.FullyConfirmedTransaction.String(); got != want {
			t.Errorf("Expected status name %s for new incoming tx, instead got %s", want, got)
		}
	}

	t.Run("ConfirmedTransaction", func(t *testing.T) {
		env.checkNextCallbackRequest(func(req *callbackRequest) {
			notification, err := req.unmarshal()
			if err != nil {
				t.Fatalf("Failed to deserialize notification data from http "+
					"callback request body: %v", err)
			}
			if notification.ID != txID {
				t.Errorf("Expected that tx id for confirmed tx in http callback "+
					"data to match id of initial tx, but they are %s %s",
					notification.ID, txID)
			}
			checkNotificationFieldsForConfirmedDeposit(t, notification)
		})

		env.websocketListeners[0].checkNextMessage(func(message []byte) {
			var event txEvent
			err := json.Unmarshal(message, &event)
			if err != nil {
				t.Fatalf("Failed to deserialize websocket notification about new deposit: %v", err)
			}
			if event.Data.ID != txID {
				t.Errorf("Expected that tx id for confirmed tx in websocket "+
					"notification will match one for initial tx, but they are %s %s",
					txID, event.Data.ID)
			}
			checkNotificationFieldsForConfirmedDeposit(t, event.Data)
		})
	})
}

func testWithdraw(t *testing.T, env *testEnvironment) {
	//http.Post(env.processingUrl("/withdraw"), "application/json")
}
