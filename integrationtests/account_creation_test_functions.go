// +build integration

package integrationtests

import (
	"context"
	"testing"

	"github.com/onederx/bitcoin-processing/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
)

func testHotWallet(t *testing.T, env *testenv.TestEnvironment, ctx context.Context) {
	var hotWalletAddress string

	runSubtest(t, "WasGenerated", func(t *testing.T) {
		var err error
		hotWalletAddress, err = env.ProcessingClient.GetHotStorageAddress()
		if err != nil {
			t.Fatalf("Failed to request hot wallet address %v", err)
		}
		if hotWalletAddress == "" {
			t.Fatalf("Hot wallet address from get_hot_storage_address API is empty")
		}
	})
	runSubtest(t, "SendFundsToHotWallet", func(t *testing.T) {
		testSendFundsToHotWallet(t, env, hotWalletAddress)
	})
	runSubtest(t, "PersistsRestart", func(t *testing.T) {
		// restart processing

		// stop
		processingContainerID := env.Processing.ID
		lastSeq := env.WebsocketListeners[0].LastSeq
		env.WebsocketListeners[0].Stop()
		env.WebsocketListeners = nil
		env.StopProcessing(ctx)
		env.WaitForContainerRemoval(ctx, processingContainerID)

		// start
		env.StartProcessingWithDefaultSettings(ctx)
		env.WaitForProcessing()

		hotWalletAddressNow, err := env.ProcessingClient.GetHotStorageAddress()
		if err != nil {
			t.Fatalf("Failed to request hot wallet address %v", err)
		}
		if hotWalletAddressNow != hotWalletAddress {
			t.Fatalf("Expected that hot wallet address after restart will be "+
				"same as before restart (%s), but it was %s", hotWalletAddress,
				hotWalletAddressNow)
		}
		// restore stopped websocket listener
		_, err = env.NewWebsocketListener(lastSeq + 1)
		if err != nil {
			t.Fatalf("Failed to connect websocket event listener %v", err)
		}
	})
}

func testGenerateClientWallet(t *testing.T, env *testenv.TestEnvironment) *wallet.Account {
	var clientAccount *wallet.Account
	runSubtest(t, "EmptyMetainfo", func(t *testing.T) {
		testGenerateClientWalletWithMetainfo(t, env, nil, 1)
	})
	runSubtest(t, "NonEmptyMetainfo", func(t *testing.T) {
		clientAccount = testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, 2)
	})
	return clientAccount
}

func testGenerateClientWalletWithMetainfo(t *testing.T, env *testenv.TestEnvironment, metainfo interface{}, checkSeq int) *wallet.Account {
	result, err := env.ProcessingClient.NewWallet(metainfo)
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
	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
	if checkSeq >= 0 {
		if got, want := event.Seq, checkSeq; got != want {
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

func testGenerateMultipleClientWallets(t *testing.T, env *testenv.TestEnvironment) []*wallet.Account {
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
