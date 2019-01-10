// +build integration

package integrationtests

import (
	"reflect"
	"testing"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
)

func getTransactionsOrFail(t *testing.T, env *testEnvironment, directionFilter, statusFilter string) []*wallet.Transaction {
	resp, err := env.processingClient.GetTransactions(&api.GetTransactionsFilter{
		Direction: directionFilter, Status: statusFilter,
	})
	if err != nil {
		t.Fatalf("API request to get transactions from processing failed: %v",
			err)
	}
	return resp
}

func testGetTransactions(t *testing.T, env *testEnvironment, clientAccount *wallet.Account) {
	runSubtest(t, "ExistingTransactions", func(t *testing.T) {
		txns := getTransactionsOrFail(t, env, "", "")

		if len(txns) == 0 {
			t.Error("Expected 'get transactions' API to return some exising " +
				"txns, instead got nothing ")
		}
	})

	deposit := testMakeDeposit(t, env, clientAccount.Address,
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.1")),
		clientAccount.Metainfo)

	// skip notifications
	env.getNextCallbackNotificationWithTimeout(t)
	env.websocketListeners[0].getNextMessageWithTimeout(t)

	checkThereIsDepositTx := func(t *testing.T, txns []*wallet.Transaction) *wallet.Transaction {
		for _, tx := range txns {
			if tx.Hash == deposit.hash {
				if tx.Address != deposit.address || tx.Amount != deposit.amount || !reflect.DeepEqual(tx.Metainfo, deposit.metainfo) {
					t.Fatalf("Expected that address, amount and metainfo of "+
						"tx returned by 'get transactions' API call will be "+
						"equal to those of created deposit (%s %s %v), but "+
						"they are %s %s %v", tx.Address, tx.Amount,
						tx.Metainfo, deposit.address, deposit.amount,
						deposit.metainfo)
				}
				return tx
			}
		}
		t.Fatalf("New transaction %s not in tx list", deposit.hash)
		return nil
	}

	runSubtest(t, "NewTx", func(t *testing.T) {
		txns := getTransactionsOrFail(t, env, "", "")
		tx := checkThereIsDepositTx(t, txns)
		deposit.id = tx.ID
	})
	runSubtest(t, "Filter", func(t *testing.T) {
		runSubtest(t, "Empty", func(t *testing.T) {
			txns := getTransactionsOrFail(t, env, "randomsomething", "")
			if len(txns) > 0 {
				t.Error("Expected that list of txns to be empty")
			}
			txns = getTransactionsOrFail(t, env, "", "nonexistent")
			if len(txns) > 0 {
				t.Error("Expected that list of txns to be empty")
			}
			txns = getTransactionsOrFail(t, env, "nonexistentdirection", "nonexistentstatus")
			if len(txns) > 0 {
				t.Error("Expected that list of txns to be empty")
			}
		})
		runSubtest(t, "Direction", func(t *testing.T) {
			txns := getTransactionsOrFail(t, env, wallet.IncomingDirection.String(), "")
			checkThereIsDepositTx(t, txns)
		})
		runSubtest(t, "Status", func(t *testing.T) {
			txns := getTransactionsOrFail(t, env, "", wallet.NewTransaction.String())
			checkThereIsDepositTx(t, txns)
		})
	})
	_, err := env.mineTx(deposit.hash)

	if err != nil {
		t.Fatal(err)
	}
	env.getNextCallbackRequestWithTimeout(t)
	env.websocketListeners[0].getNextMessageWithTimeout(t)
}
