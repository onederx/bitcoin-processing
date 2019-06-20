// +build integration

package integrationtests

import (
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
)

func testDepositAndWithdrawMultipleMixed(t *testing.T, env *testenv.TestEnvironment, accounts []*wallet.Account) {
	balanceByNow := getStableBalanceOrFail(t, env)

	deposits := testTxCollection{
		testMakeDeposit(t, env, accounts[0].Address,
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.017")), accounts[0].Metainfo),
		testMakeDeposit(t, env, testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, -1).Address,
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.06")), initialTestMetainfo),
	}
	withdrawals := testTxCollection{
		testMakeWithdraw(t, env, getNewAddressForWithdrawOrFail(t, env),
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.03")), initialTestMetainfo),
		testMakeWithdraw(t, env, getNewAddressForWithdrawOrFail(t, env),
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0095")), initialTestMetainfo),
	}

	cbNotifications, wsEvents := collectNotificationsAndEvents(t, env, 4)
	depositEventNotifications := findAllWsNotificationsWithTypeOrFail(t, wsEvents, events.NewIncomingTxEvent, 2)
	withdrawEventNotifications := findAllWsNotificationsWithTypeOrFail(t, wsEvents, events.NewOutgoingTxEvent, 2)

	for _, tx := range deposits {
		n := findNotificationForTxOrFail(t, cbNotifications, tx)
		checkNotificationFieldsForNewDeposit(t, n, tx)
		tx.id = n.ID
		wsN := findNotificationForTxOrFail(t, depositEventNotifications, tx)
		checkNotificationFieldsForNewDeposit(t, wsN, tx)
	}

	for _, tx := range withdrawals {
		n := findNotificationForTxOrFail(t, cbNotifications, tx)
		checkNotificationFieldsForNewClientWithdraw(t, n, tx)
		tx.hash = n.Hash
		wsN := findNotificationForTxOrFail(t, withdrawEventNotifications, tx)
		checkNotificationFieldsForNewClientWithdraw(t, wsN, tx)
	}

	append(deposits, withdrawals...).mineOrFail(t, env)

	newDeposit := testMakeDeposit(t, env, deposits[1].address,
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.033")), deposits[1].metainfo)

	newWithdraw := testMakeWithdraw(t, env, withdrawals[0].address,
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0278")), nil)

	cbNotifications, wsEvents = collectNotificationsAndEvents(t, env, 6)

	confirmedDepositNotifications := findAllWsNotificationsWithTypeOrFail(t, wsEvents, events.IncomingTxConfirmedEvent, 2)
	confirmedWithdrawNotifications := findAllWsNotificationsWithTypeOrFail(t, wsEvents, events.OutgoingTxConfirmedEvent, 2)

	for _, tx := range deposits {
		n := findNotificationForTxOrFail(t, cbNotifications, tx)
		checkNotificationFieldsForFullyConfirmedDeposit(t, n, tx)
		tx.id = n.ID
		wsN := findNotificationForTxOrFail(t, confirmedDepositNotifications, tx)
		checkNotificationFieldsForFullyConfirmedDeposit(t, wsN, tx)
	}

	for _, tx := range withdrawals {
		n := findNotificationForTxOrFail(t, cbNotifications, tx)
		checkNotificationFieldsForFullyConfirmedClientWithdraw(t, n, tx)
		tx.hash = n.Hash
		wsN := findNotificationForTxOrFail(t, confirmedWithdrawNotifications, tx)
		checkNotificationFieldsForFullyConfirmedClientWithdraw(t, wsN, tx)
	}

	newDepositNotifications := findAllWsNotificationsWithTypeOrFail(t, wsEvents, events.NewIncomingTxEvent, 1)
	newWithdrawNotifications := findAllWsNotificationsWithTypeOrFail(t, wsEvents, events.NewOutgoingTxEvent, 1)

	n := findNotificationForTxOrFail(t, cbNotifications, newDeposit)
	checkNotificationFieldsForNewDeposit(t, n, newDeposit)
	newDeposit.id = n.ID
	checkNotificationFieldsForNewDeposit(t, newDepositNotifications[0], newDeposit)

	n = findNotificationForTxOrFail(t, cbNotifications, newWithdraw)
	checkNotificationFieldsForNewClientWithdraw(t, n, newWithdraw)
	newWithdraw.hash = n.Hash
	checkNotificationFieldsForNewClientWithdraw(t, newWithdrawNotifications[0], newWithdraw)

	// balance after first txns, minus amount of new withdraw, because
	// unconfirmed withdrawals are still subtracted from confirmed balance by
	// bitcoin node
	expectedConfBalanceByNow := balanceByNow + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0097"))

	balanceAfterAllTxns := balanceByNow + bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.0427"))

	checkBalance(t, env, expectedConfBalanceByNow, balanceAfterAllTxns)

	testTxCollection{newDeposit, newWithdraw}.mineOrFail(t, env)

	cbNotifications, wsEvents = collectNotificationsAndEvents(t, env, 2)

	n = findNotificationForTxOrFail(t, cbNotifications, newDeposit)
	checkNotificationFieldsForFullyConfirmedDeposit(t, n, newDeposit)
	checkNotificationFieldsForFullyConfirmedDeposit(t,
		findEventWithTypeOrFail(t, wsEvents, events.IncomingTxConfirmedEvent).Data.(*wallet.TxNotification),
		newDeposit)

	n = findNotificationForTxOrFail(t, cbNotifications, newWithdraw)
	checkNotificationFieldsForFullyConfirmedClientWithdraw(t, n, newWithdraw)
	checkNotificationFieldsForFullyConfirmedClientWithdraw(t,
		findEventWithTypeOrFail(t, wsEvents, events.OutgoingTxConfirmedEvent).Data.(*wallet.TxNotification),
		newWithdraw)

	checkBalance(t, env, balanceAfterAllTxns, balanceAfterAllTxns)
}
