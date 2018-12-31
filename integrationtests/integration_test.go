// +build integration

package integrationtests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
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
		if respData["address"].(string) == "" {
			t.Fatalf("Generated address is empty")
		}
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
		if respData["address"].(string) == "" {
			t.Fatalf("Generated address is empty")
		}
	})
}
