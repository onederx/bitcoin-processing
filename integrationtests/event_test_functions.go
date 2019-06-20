// +build integration

package integrationtests

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
	"github.com/onederx/bitcoin-processing/util"
)

func testProcessingCatchesMissedEvents(t *testing.T, env *testenv.TestEnvironment, ctx context.Context, accounts []*wallet.Account) {
	withdraw := testMakeWithdraw(t, env, getNewAddressForWithdrawOrFail(t, env),
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.088")), nil)

	n := env.GetNextCallbackNotificationWithTimeout(t)
	if n.ID != withdraw.id {
		t.Fatalf("Expected that notification will correspond to tx %s, but "+
			"got %s", withdraw.id, n.ID)
	}
	withdraw.hash = n.Hash
	// skip websocket notification
	env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

	// stop processing
	env.WebsocketListeners[0].Stop()
	env.WebsocketListeners = nil

	processingContainerID := env.Processing.ID
	env.StopProcessing(ctx)
	env.WaitForContainerRemoval(ctx, processingContainerID)

	deposits := testTxCollection{
		testMakeDeposit(t, env, accounts[0].Address,
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.041")), accounts[0].Metainfo),
		testMakeDeposit(t, env, accounts[1].Address,
			bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.96")), accounts[1].Metainfo),
	}
	append(deposits, withdraw).mineOrFail(t, env)

	env.StartProcessingWithDefaultSettings(ctx)

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
			httpNotification := env.GetNextCallbackNotificationWithTimeout(t)
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
			listener, err := env.NewWebsocketListener(0)
			if err != nil {
				t.Fatalf("Failed to connect websocket event listener %v", err)
			}
			allMessages := make([]*events.NotificationWithSeq, 0, maxSeq)
			if err != nil {
				t.Fatal(err)
			}
			for {
				msg := listener.GetNextMessageWithTimeout(t)
				allMessages = append(allMessages, msg)
				if msg.Seq == maxSeq {
					break
				}
			}
			testWebsocketEvents(t, allMessages[len(allMessages)-5:])
		})
		runSubtest(t, "GetOnlyNeededNotifications", func(t *testing.T) {
			listener, err := env.NewWebsocketListener(maxSeq - 4)
			if err != nil {
				t.Fatalf("Failed to connect websocket event listener %v", err)
			}
			neededMessages := make([]*events.NotificationWithSeq, 0, 5)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 5; i++ {
				msg := listener.GetNextMessageWithTimeout(t)
				neededMessages = append(neededMessages, msg)
			}
			if neededMessages[len(neededMessages)-1].Seq != maxSeq {
				t.Fatalf("Expected last websocket message to have seq %d, "+
					"instead it is %d", maxSeq, neededMessages[len(neededMessages)-1].Seq)
			}
			testWebsocketEvents(t, neededMessages)
			listener.Stop()
			env.WebsocketListeners = []*testenv.WebsocketListener{env.WebsocketListeners[0]}
		})
	})
}

func testWebsocketListeners(t *testing.T, env *testenv.TestEnvironment) {
	runSubtest(t, "ExistingEvents", func(t *testing.T) {
		listener, err := env.NewWebsocketListener(0)
		if err != nil {
			t.Fatalf("Failed to connect websocket event listener %v", err)
		}

		event := listener.GetNextMessageWithTimeout(t)

		if event == nil {
			t.Fatal("Expected existing event to be non-nil")
		}
		listener.Stop()
		env.WebsocketListeners = []*testenv.WebsocketListener{env.WebsocketListeners[0]}
	})
	runSubtest(t, "GetEventsFromSeq", func(t *testing.T) {
		runSubtest(t, "Sanity", func(t *testing.T) {
			seq := env.WebsocketListeners[0].LastSeq - 10
			listener, err := env.NewWebsocketListener(seq)
			if err != nil {
				t.Fatalf("Failed to connect websocket event listener %v", err)
			}
			event := listener.GetNextMessageWithTimeout(t)

			if event == nil {
				t.Fatal("Expected existing event to be non-nil")
			}
			if event.Seq < seq {
				t.Errorf("Expected event requested from seq to have seq >= %d, "+
					"but received %d", seq, event.Seq)
			}
			listener.Stop()
			env.WebsocketListeners = []*testenv.WebsocketListener{env.WebsocketListeners[0]}
		})
		runSubtest(t, "GetLostEvents", func(t *testing.T) {
			// create a couple of events that listener will catch in time
			account1 := testGenerateClientWalletWithMetainfo(t, env,
				initialTestMetainfo, env.WebsocketListeners[0].LastSeq+1)
			deposit1 := testMakeDeposit(t, env, account1.Address, testDepositAmount,
				account1.Metainfo)

			deposit1.id = env.GetNextCallbackNotificationWithTimeout(t).ID
			event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
			lastSeq := event.Seq
			if got, want := event.Type, events.NewIncomingTxEvent; got != want {
				t.Errorf("Unexpected event type for new deposit, wanted %s, got %s:",
					want, got)
			}
			checkNotificationFieldsForNewDeposit(t, event.Data.(*wallet.TxNotification), deposit1)

			// stop this listener to simulate situation that client has gone away
			// while some events are happening
			env.WebsocketListeners[0].Stop()
			env.WebsocketListeners = nil

			// (this withdraw will be put on hold until manual confirmation
			// because it's amount is 0.4)
			withdraw := testMakeWithdraw(t, env,
				getNewAddressForWithdrawOrFail(t, env),
				bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.4")), nil)

			// skip http callback notification about this withdraw
			env.GetNextCallbackNotificationWithTimeout(t)

			account2, err := env.ProcessingClient.NewWallet(nil)
			if err != nil {
				t.Fatal(err)
			}
			deposit1.mineOrFail(t, env)

			// skip http callback notification about deposit confirmation
			env.GetNextCallbackNotificationWithTimeout(t)

			// ok, now client is going back online
			listener, err := env.NewWebsocketListener(lastSeq + 1)

			withdrawEvent := listener.GetNextMessageWithTimeout(t)

			if withdrawEvent.Type != events.PendingStatusUpdatedEvent {
				t.Errorf("Unexpected event type: wanted %s, got %s",
					events.PendingStatusUpdatedEvent, withdrawEvent.Type)
			}
			checkNotificationFieldsForWithdrawPendingManualConfirmation(t,
				withdrawEvent.Data.(*wallet.TxNotification), withdraw)

			newAccountEvent := listener.GetNextMessageWithTimeout(t)

			if got, want := newAccountEvent.Type, events.NewAddressEvent; got != want {
				t.Errorf("Unexpected event type for new wallet generation, wanted %s, got %s:",
					want, got)
			}
			newAccountEventData := newAccountEvent.Data.(*wallet.Account)
			if got, want := newAccountEventData.Address, account2.Address; got != want {
				t.Errorf("Expected address to be %s, but instead got %s",
					want, got)
			}
			if newAccountEventData.Metainfo != nil {
				t.Errorf("Expected metainfo to be nil, but instead got %v",
					newAccountEventData.Metainfo)
			}

			deposit1ConfirmedEvent := listener.GetNextMessageWithTimeout(t)

			if got, want := deposit1ConfirmedEvent.Type, events.IncomingTxConfirmedEvent; got != want {
				t.Errorf("Unexpected event type: wanted %s, got %s", want, got)
			}
			checkNotificationFieldsForFullyConfirmedDeposit(t,
				deposit1ConfirmedEvent.Data.(*wallet.TxNotification), deposit1)
		})
	})
	runSubtest(t, "ParallelListeners", func(t *testing.T) {
		seq := env.WebsocketListeners[0].LastSeq + 1
		listener1, err := env.NewWebsocketListener(seq)
		if err != nil {
			t.Fatalf("Failed to connect websocket event listener %v", err)
		}
		listener2, err := env.NewWebsocketListener(seq)
		if err != nil {
			t.Fatalf("Failed to connect websocket event listener %v", err)
		}

		account := testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, seq)

		event1 := listener1.GetNextMessageWithTimeout(t)
		event2 := listener2.GetNextMessageWithTimeout(t)

		if event1.Seq != seq {
			t.Errorf("Expected next notification seqnum be %d, but it is %d",
				seq, event1.Seq)
		}
		if got, want := event1.Type, events.NewAddressEvent; got != want {
			t.Errorf("Unexpected event type for new wallet generation, wanted %s, got %s:",
				want, got)
		}
		data := event1.Data.(*wallet.Account)
		if got, want := data.Address, account.Address; got != want {
			t.Errorf("Expected address from WS notification to be equal "+
				"to address from API response (%s), but instead got %s",
				want, got)
		}
		compareMetainfo(t, data.Metainfo, initialTestMetainfo)

		if !reflect.DeepEqual(event1, event2) {
			t.Errorf("Expected notifications from 2 websocket listeners to be"+
				"identical, but they are %v %v", event1, event2)
		}
		listener1.Stop()
		listener2.Stop()
		env.WebsocketListeners = []*testenv.WebsocketListener{env.WebsocketListeners[0]}
	})
}

func testGetEvents(t *testing.T, env *testenv.TestEnvironment) {
	var lastSeq int
	runSubtest(t, "ExistingEvents", func(t *testing.T) {
		evts, err := env.ProcessingClient.GetEvents(0)
		if err != nil {
			t.Fatalf("API call /get_events with seq 0 failed: %v", err)
		}
		if len(evts) == 0 {
			t.Fatal("Expected /get_events call to return some existing " +
				"events, instead got nothing")
		}
		lastSeq = evts[len(evts)-1].Seq
	})
	runSubtest(t, "GetEventsFromSeq", func(t *testing.T) {
		seq := lastSeq - 10

		evts, err := env.ProcessingClient.GetEvents(seq)
		if err != nil {
			t.Fatalf("API call /get_events with seq %d failed: %v", seq, err)
		}
		if len(evts) == 0 {
			t.Fatal("Expected /get_events call to return some events, " +
				"instead got nothing")
		}

		if evts[0].Seq < seq {
			t.Errorf("Expected event requested from seq to have seq >= %d, "+
				"but received %d", seq, evts[0].Seq)
		}
	})
	runSubtest(t, "NewEvent", func(t *testing.T) {
		seq := lastSeq + 1
		account := testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, seq)
		evts, err := env.ProcessingClient.GetEvents(seq)
		if err != nil {
			t.Fatalf("API call /get_events with seq %d failed: %v",
				seq, err)
		}
		if len(evts) == 0 {
			t.Fatal("Expected /get_events call to return new event, instead " +
				"got nothing")
		}
		event := evts[0]
		if event.Seq != seq {
			t.Errorf("Expected next notification seqnum be %d, but it is %d",
				seq, event.Seq)
		}
		if got, want := event.Type, events.NewAddressEvent; got != want {
			t.Errorf("Unexpected event type for new wallet generation, wanted %s, got %s:",
				want, got)
		}
		data := event.Data.(*wallet.Account)
		if got, want := data.Address, account.Address; got != want {
			t.Errorf("Expected address from WS notification to be equal "+
				"to address from API response (%s), but instead got %s",
				want, got)
		}
		compareMetainfo(t, data.Metainfo, initialTestMetainfo)
	})
}

func testHTTPCallbackBackoff(t *testing.T, env *testenv.TestEnvironment, clientAccount *wallet.Account) {
	errorCount := 3

	env.CallbackHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		if errorCount > 1 {
			errorCount--
		} else {
			env.CallbackHandler = nil
		}
	}

	deposit := testMakeDeposit(t, env, clientAccount.Address,
		bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("0.076")),
		clientAccount.Metainfo)

	n := env.GetNextCallbackNotificationWithTimeout(t)
	deposit.id = n.ID
	checkNotificationFieldsForNewDeposit(t, n, deposit)
	deposit.mineOrFail(t, env)
	n = env.GetNextCallbackNotificationWithTimeout(t)
	checkNotificationFieldsForFullyConfirmedDeposit(t, n, deposit)

	// skip websocket notifications about deposit
	for i := 0; i < 2; i++ {
		env.WebsocketListeners[0].GetNextMessageWithTimeout(t)
	}
}
