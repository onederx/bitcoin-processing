// +build integration

package integrationtests

import (
	"context"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/util"
)

func testProcessingCatchesMissedEvents(t *testing.T, env *testEnvironment, ctx context.Context, accounts []*wallet.Account) {
	withdraw := testMakeWithdraw(t, env, getNewAddressForWithdrawOrFail(t, env),
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.088")), nil)

	n := env.getNextCallbackNotificationWithTimeout(t)
	if n.ID != withdraw.id {
		t.Fatalf("Expected that notification will correspond to tx %s, but "+
			"got %s", withdraw.id, n.ID)
	}
	withdraw.hash = n.Hash
	// skip websocket notification
	env.websocketListeners[0].getNextMessageWithTimeout(t)

	// stop processing
	env.websocketListeners[0].stop()
	env.websocketListeners = nil
	env.stopProcessing(ctx)

	deposits := testTxCollection{
		testMakeDeposit(t, env, accounts[0].Address,
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.041")), accounts[0].Metainfo),
		testMakeDeposit(t, env, accounts[1].Address,
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.96")), accounts[1].Metainfo),
	}
	append(deposits, withdraw).mineOrFail(t, env)

	env.startProcessingWithDefaultSettings(ctx)

	maxSeq := 0

	testNotificationsAboutMissedEvents := func(t *testing.T, depositNotifications map[string][]*wallet.TxNotification, withdrawNotification *wallet.TxNotification) {
		for _, tx := range deposits {
			notifications := depositNotifications[tx.hash]

			if len(notifications) != 2 {
				t.Fatal("Expected 2 notifications about a deposit to arrive: 1 "+
					"about new tx and 1 about confirmation. instead got %d",
					len(notifications))
			}
			if notifications[0].StatusCode != 0 || notifications[0].StatusStr != wallet.NewTransaction.String() {
				t.Fatal("Notification order error: expected notification about " +
					"new tx to come before notification about confirmatios")
			}
			checkNotificationFieldsForNewDeposit(t, notifications[0], tx)
			tx.id = notifications[0].ID
			checkNotificationFieldsForFullyConfirmedDeposit(t, notifications[1], tx)
		}
		checkNotificationFieldsForFullyConfirmedClientWithdraw(t, withdrawNotification, withdraw)
	}

	runSubtest(t, "HTTPCallbackNotifications", func(t *testing.T) {
		var withdrawNotification *wallet.TxNotification
		depositNotifications := make(map[string][]*wallet.TxNotification)

		for i := 0; i < 5; i++ {
			httpNotification := env.getNextCallbackNotificationWithTimeout(t)
			maxSeq = util.Max(maxSeq, httpNotification.Seq)

			switch httpNotification.Hash {
			case withdraw.hash:
				if withdrawNotification != nil {
					t.Error("Expected only 1 notification for withdraw tx, got more")
				}
				withdrawNotification = httpNotification
			case deposits[0].hash:
				fallthrough
			case deposits[1].hash:
				depositNotifications[httpNotification.Hash] = append(
					depositNotifications[httpNotification.Hash], httpNotification)
			default:
				t.Errorf("Unexpected notification for tx %s", httpNotification.Hash)
			}
		}
		testNotificationsAboutMissedEvents(t, depositNotifications, withdrawNotification)
	})
	testWebsocketEvents := func(t *testing.T, evts []*events.NotificationWithSeq) {
		var withdrawNotification *wallet.TxNotification
		depositNotifications := make(map[string][]*wallet.TxNotification)
		for _, ev := range evts {
			data := ev.Data.(*wallet.TxNotification)
			switch ev.Type {
			case events.OutgoingTxConfirmedEvent:
				withdrawNotification = data
			case events.NewIncomingTxEvent:
				if depositNotifications[data.Hash] != nil {
					t.Error("Websocket event order error: expected event " +
						"about new tx to come before event about confirmation")
				}
				fallthrough
			case events.IncomingTxConfirmedEvent:
				depositNotifications[data.Hash] = append(
					depositNotifications[data.Hash], data)
			}
		}
		testNotificationsAboutMissedEvents(t, depositNotifications, withdrawNotification)
	}
	runSubtest(t, "WebsocketNotifications", func(t *testing.T) {
		runSubtest(t, "GetAllNotifications", func(t *testing.T) {
			listener, err := env.newWebsocketListener(0)
			allMessages := make([]*events.NotificationWithSeq, 0, maxSeq)
			if err != nil {
				t.Fatal(err)
			}
			for {
				msg := listener.getNextMessageWithTimeout(t)
				allMessages = append(allMessages, msg)
				if msg.Seq == maxSeq {
					break
				}
			}
			testWebsocketEvents(t, allMessages[len(allMessages)-5:])
		})
		runSubtest(t, "GetOnlyNeededNotifications", func(t *testing.T) {
			listener, err := env.newWebsocketListener(maxSeq - 4)
			neededMessages := make([]*events.NotificationWithSeq, 0, 5)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 5; i++ {
				msg := listener.getNextMessageWithTimeout(t)
				neededMessages = append(neededMessages, msg)
			}
			if neededMessages[len(neededMessages)-1].Seq != maxSeq {
				t.Fatalf("Expected last websocket message to have seq %d, "+
					"instead it is %d", maxSeq, neededMessages[len(neededMessages)-1].Seq)
			}
			testWebsocketEvents(t, neededMessages)
			listener.stop()
			env.websocketListeners = []*websocketListener{env.websocketListeners[0]}
		})
	})
}
