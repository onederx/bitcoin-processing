// +build integration

package integrationtests

import (
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

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
