// +build integration

package integrationtests

import (
	"context"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/wallet"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
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
	env, err := testenv.NewTestEnvironment(ctx, depositFee)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Stop(ctx)
	env.WaitForLoad()
	err = env.StartProcessingWithDefaultSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.StopProcessing(ctx)
	env.WaitForProcessing()

	_, err = env.ProcessingClient.GetEvents(0)

	if err != nil {
		t.Fatal(err)
	}
}

func TestCommonUsage(t *testing.T) {
	ctx := context.Background()
	env, err := testenv.NewTestEnvironment(ctx, depositFee)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Stop(ctx)
	env.WaitForLoad()
	err = env.StartProcessingWithDefaultSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.StopProcessing(ctx)
	env.WaitForProcessing()
	_, err = env.NewWebsocketListener(0)
	if err != nil {
		t.Fatalf("Failed to connect websocket event listener %v", err)
	}
	runSubtest(t, "InitialCheckBalanceGivesZero", func(t *testing.T) {
		checkBalance(t, env, zeroBTC, zeroBTC)
	})
	runSubtest(t, "InitialRequiredFromColdStorageGivesZero", func(t *testing.T) {
		checkRequiredFromColdStorage(t, env, zeroBTC)
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
	runSubtest(t, "HotWallet", func(t *testing.T) {
		testHotWallet(t, env, ctx)
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
	runSubtest(t, "MissedEvents", func(t *testing.T) {
		testProcessingCatchesMissedEvents(t, env, ctx, accounts)
	})
	runSubtest(t, "WebsocketListeners", func(t *testing.T) {
		testWebsocketListeners(t, env)
	})
	runSubtest(t, "GetEvents", func(t *testing.T) {
		testGetEvents(t, env)
	})
	runSubtest(t, "HTTPCallbackBackoff", func(t *testing.T) {
		testHTTPCallbackBackoff(t, env, clientAccount)
	})
	runSubtest(t, "GetTransactions", func(t *testing.T) {
		testGetTransactions(t, env, clientAccount)
	})
	runSubtest(t, "WithdrawToColdStorage", func(t *testing.T) {
		testWithdrawToColdStorage(t, env, ctx)
	})
}

func TestMoreConfirmations(t *testing.T) {
	const neededConfirmations = 4

	ctx := context.Background()
	env, err := testenv.NewTestEnvironment(ctx, depositFee)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Stop(ctx)
	env.WaitForLoad()

	processingSettings := testenv.DefaultSettings

	processingSettings.MaxConfirmations = neededConfirmations
	processingSettings.CallbackURL = env.CallbackURL

	err = env.StartProcessing(ctx, &processingSettings)
	if err != nil {
		t.Fatal(err)
	}
	defer env.StopProcessing(ctx)
	env.WaitForProcessing()
	_, err = env.NewWebsocketListener(0)
	if err != nil {
		t.Fatalf("Failed to connect websocket event listener %v", err)
	}

	clientWallet, err := env.ProcessingClient.NewWallet(nil)

	// skip new wallet notification
	env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

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

func TestMultipleExits(t *testing.T) {
	ctx := context.Background()
	env, err := testenv.NewTestEnvironment(ctx, depositFee)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Stop(ctx)
	env.WaitForLoad()
	err = env.StartProcessingWithDefaultSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.StopProcessing(ctx)
	env.WaitForProcessing()
	_, err = env.NewWebsocketListener(0)
	if err != nil {
		t.Fatalf("Failed to connect websocket event listener %v", err)
	}

	// create several accounts (this function actually always returns 3)
	accounts := testGenerateMultipleClientWallets(t, env)
	testDepositSingleBitcoinTxWithMultipleExists(t, env, accounts)
}
