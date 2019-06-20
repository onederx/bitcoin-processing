// +build integration

package integrationtests

import (
	"reflect"
	"testing"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/wallet"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
	wallettypes "github.com/onederx/bitcoin-processing/wallet/types"
)

func getTransactionsOrFail(t *testing.T, env *testenv.TestEnvironment, directionFilter, statusFilter string) []*wallettypes.Transaction {
	resp, err := env.ProcessingClient.GetTransactions(&api.GetTransactionsFilter{
		Direction: directionFilter, Status: statusFilter,
	})
	if err != nil {
		t.Fatalf("API request to get transactions from processing failed: %v",
			err)
	}
	return resp
}

func testGetTransactions(t *testing.T, env *testenv.TestEnvironment, clientAccount *wallet.Account) {
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
	env.GetNextCallbackNotificationWithTimeout(t)
	env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

	checkThereIsDepositTx := func(t *testing.T, txns []*wallettypes.Transaction) *wallettypes.Transaction {
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
			txns := getTransactionsOrFail(t, env, wallettypes.IncomingDirection.String(), "")
			checkThereIsDepositTx(t, txns)
		})
		runSubtest(t, "Status", func(t *testing.T) {
			txns := getTransactionsOrFail(t, env, "", wallettypes.NewTransaction.String())
			checkThereIsDepositTx(t, txns)
		})
	})
	deposit.mineOrFail(t, env)
	env.GetNextCallbackRequestWithTimeout(t)
	env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
}
