// +build integration

package integrationtests

import (
	"context"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
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

func testDepositAndWithdrawMultipleMixed(t *testing.T, env *testEnvironment, accounts []*wallet.Account) {
	// TODO
}
