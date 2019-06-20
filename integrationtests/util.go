// +build integration

package integrationtests

import (
	"net"
	"runtime/debug"
	"testing"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/api"
	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv/pgmitm"
	"github.com/onederx/bitcoin-processing/integrationtests/util"
	wallettypes "github.com/onederx/bitcoin-processing/wallet/types"
)

func findNotificationForTxOrFail(t *testing.T, notifications []*wallettypes.TxNotification, tx *txTestData) *wallettypes.TxNotification {
	for _, n := range notifications {
		if tx.id != uuid.Nil {
			if n.ID == tx.id {
				return n
			}
		} else if n.Hash == tx.hash && n.Amount == tx.amount && n.Address == tx.address {
			return n
		}
	}
	t.Helper()
	t.Fatalf("Failed to find relevant notification for tx id %s hash %s "+
		"address %s amount %s.", tx.id, tx.hash,
		tx.address, tx.amount)
	return nil
}

// runSubtest is the same as t.Run, but turns panic into t.Fatal
func runSubtest(t *testing.T, name string, f func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Test %s failed with panic: %v. Stack %s", name, r,
					debug.Stack())
			}
		}()
		f(t)
	})
}

func waitForEventOrFailTest(t *testing.T, callback func() error) {
	err := util.WaitForEvent(callback)
	if err != nil {
		t.Helper()
		t.Fatal(err)
	}
}

func getStableClientBalanceOrFail(t *testing.T, env *testenv.TestEnvironment) bitcoin.BTCAmount {
	clientBalance, err := env.GetClientBalance()

	if err != nil {
		t.Helper()
		t.Fatal(err)
	}

	return stableBalanceOrFail(t, "client balance", clientBalance)
}

func getStableBalanceOrFail(t *testing.T, env *testenv.TestEnvironment) bitcoin.BTCAmount {
	balanceInfo, err := env.ProcessingClient.GetBalance()

	if err != nil {
		t.Helper()
		t.Fatal(err)
	}

	return stableBalanceOrFail(t, "balance", balanceInfo)
}

func getNewAddressForWithdrawOrFail(t *testing.T, env *testenv.TestEnvironment) string {
	addressDecoded, err := env.Regtest["node-client"].NodeAPI.CreateNewAddress()

	if err != nil {
		t.Helper()
		t.Fatalf("Failed to request new address from client node: %v", err)
	}
	return addressDecoded
}

func collectNotificationsAndEvents(t *testing.T, env *testenv.TestEnvironment, n int) (httpNotifications []*wallettypes.TxNotification, wsEvents []*events.NotificationWithSeq) {
	t.Helper()
	for i := 0; i < n; i++ {
		httpNotifications = append(httpNotifications, env.GetNextCallbackNotificationWithTimeout(t))
		wsEvents = append(wsEvents, env.WebsocketListeners[0].GetNextMessageWithTimeout(t))
	}
	return
}

func findAllWsNotificationsWithTypeOrFail(t *testing.T, evts []*events.NotificationWithSeq, et events.EventType, n int) (wsNotifications []*wallettypes.TxNotification) {
	t.Helper()
	for _, event := range findAllEventsWithTypeOrFail(t, evts, et, n) {
		wsNotifications = append(wsNotifications, event.Data.(*wallettypes.TxNotification))
	}
	return
}

func findEventWithTypeOrFail(t *testing.T, evts []*events.NotificationWithSeq, et events.EventType) *events.NotificationWithSeq {
	for _, e := range evts {
		if e.Type == et {
			return e
		}
	}
	t.Helper()
	t.Fatalf("Failed to find event with requested type %s", et)
	return nil
}

func findAllEventsWithTypeOrFail(t *testing.T, evts []*events.NotificationWithSeq, et events.EventType, n int) (result []*events.NotificationWithSeq) {
	for _, e := range evts {
		if e.Type == et {
			result = append(result, e)
		}
		if len(result) >= n {
			return
		}
	}
	t.Helper()
	t.Fatalf("Failed to find %d events with requested type %s", n, et)
	return nil
}

func collectNotifications(t *testing.T, env *testenv.TestEnvironment, eventType events.EventType, n int) (httpNotifications []*wallettypes.TxNotification, wsNotifications []*wallettypes.TxNotification) {
	t.Helper()
	for i := 0; i < n; i++ {
		httpNotifications = append(httpNotifications, env.GetNextCallbackNotificationWithTimeout(t))
		event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

		if got, want := event.Type, eventType; got != want {
			t.Fatalf("Unexpected event type for event, wanted %s, got %s:",
				want, got)
		}
		wsNotifications = append(wsNotifications, event.Data.(*wallettypes.TxNotification))
	}
	return
}

func stableBalanceOrFail(t *testing.T, name string, bal *api.BalanceInfo) bitcoin.BTCAmount {
	if bal.Balance != bal.BalanceWithUnconf {
		t.Helper()
		t.Fatalf("Expected that confirmed and uncofirmed %s to be equal "+
			"by this moment, but they are %s %s", name, bal.Balance,
			bal.BalanceWithUnconf)
	}
	return bal.Balance
}

func addPgMITMOrFail(env *testenv.TestEnvironment, settings *testenv.ProcessingSettings, t *testing.T, allowUpstreamConnFailure bool) *pgmitm.PgMITM {
	mitm, err := pgmitm.NewPgMITM(
		env.NetworkGateway+":",
		env.DB.IP+":5432",
		allowUpstreamConnFailure,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = mitm.Start()

	if err != nil {
		t.Fatal(err)
	}
	mitmAddr := mitm.Addr()

	settings.PostgresAddress, settings.PostgresPort, err = net.SplitHostPort(mitmAddr)

	if err != nil {
		t.Fatal(err)
	}
	return mitm
}
