// +build integration

package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

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

	resp, err := http.Get(env.processingUrl("/get_events"))

	getGoodResponseResultOrFail(t, resp, err)
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
	t.Run("GenerateClientWallet", func(t *testing.T) {
		testGenerateClientWallet(t, env)
	})
}

func testHotWalletGenerated(t *testing.T, env *testEnvironment) {
	resp, err := http.Get(env.processingUrl("/get_hot_storage_address"))
	respResult := getGoodResponseResultOrFail(t, resp, err)
	hotWalletAddress, ok := respResult.(string)
	if !ok {
		t.Fatalf("Hot wallet address from get_hot_storage_address API is not a string: %v", respResult)
	}
	if hotWalletAddress == "" {
		t.Fatalf("Hot wallet address from get_hot_storage_address API is empty")
	}
}

func testGenerateClientWallet(t *testing.T, env *testEnvironment) {
	type newWalletEvent struct {
		Type events.EventType `json:"type"`
		Seq  int              `json:"seq"`
		Data wallet.Account   `json:"data"`
	}
	t.Run("EmptyMetainfo", func(t *testing.T) {
		resp, err := http.Post(
			env.processingUrl("/new_wallet"),
			"application/json",
			nil,
		)
		respResult := getGoodResponseResultOrFail(t, resp, err)
		respData := respResult.(map[string]interface{})
		if respData["metainfo"] != nil {
			t.Fatalf("Metainfo unexpectedly non-nil: %v", respData["metainfo"])
		}
		address := respData["address"].(string)
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
		type testMetainfo struct {
			Testing string `json:"testing"`
			Index   int    `json:"index"`
			Data    struct {
				User string `json:"user"`
			} `json:"data"`
		}
		initialTestMetainfo := testMetainfo{
			Testing: "testtest",
			Index:   123,
		}
		initialTestMetainfo.Data.User = "tester"
		initialTestMetainfoJSON, err := json.Marshal(initialTestMetainfo)
		resp, err := http.Post(
			env.processingUrl("/new_wallet"),
			"application/json",
			bytes.NewReader(initialTestMetainfoJSON),
		)
		respResult := getGoodResponseResultOrFail(t, resp, err)
		respData := respResult.(map[string]interface{})
		if respData["metainfo"] == nil {
			t.Fatalf("Metainfo unexpectedly nil")
		}
		compareMetainfo(t, respData["metainfo"], initialTestMetainfo)
		address := respData["address"].(string)
		if address == "" {
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
			if got, want := event.Data.Address, address; got != want {
				t.Errorf("Expected address from WS notification to be equal "+
					"to address from API response (%s), but instead got %s",
					want, got)
			}
			compareMetainfo(t, event.Data.Metainfo, initialTestMetainfo)
		})
	})
}
