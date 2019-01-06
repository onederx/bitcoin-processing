package integrationtests

import (
	"fmt"
	"net"
	"os"
	"path"
	"testing"
	"time"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
)

const waitForEventRetries = 120

func getFullSourcePath(dirName string) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(path.Dir(cwd), dirName)
}

func waitForEventOrPanic(callback func() error) {
	err := waitForEvent(callback)
	if err != nil {
		panic(err)
	}
}

func waitForEventOrFailTest(t *testing.T, callback func() error) {
	err := waitForEvent(callback)
	if err != nil {
		t.Fatal(err)
	}
}

func waitForEvent(callback func() error) error {
	retries := waitForEventRetries

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		err := callback()
		if err != nil {
			retries--
			if retries <= 0 {
				return err
			}
		} else {
			return nil
		}
	}
	return nil
}

func waitForPort(host string, port uint16) {
	waitForEventOrPanic(func() error {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	})
}

func getNewAddressForWithdrawOrFail(t *testing.T, env *testEnvironment) string {
	addressDecoded, err := env.regtest["node-client"].nodeAPI.CreateNewAddress()

	if err != nil {
		t.Fatalf("Failed to request new address from client node: %v", err)
	}
	return addressDecoded.EncodeAddress()
}

// runSubtest is the same as t.Run, but turns panic into t.Fatal
func runSubtest(t *testing.T, name string, f func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Test %s failed with panic: %v", name, r)
			}
		}()
		f(t)
	})
}

func findNotificationForTxOrFail(t *testing.T, notifications []*wallet.TxNotification, tx *txTestData) *wallet.TxNotification {
	for _, n := range notifications {
		if tx.id != uuid.Nil {
			if n.ID == tx.id {
				return n
			}
		} else if n.Hash == tx.hash && n.Amount == tx.amount && n.Address == tx.address {
			return n
		}
	}
	t.Fatal("Failed to relevant notification")
	return nil
}

func findEventWithTypeOrFail(t *testing.T, evts []*events.NotificationWithSeq, et events.EventType) *events.NotificationWithSeq {
	for _, e := range evts {
		if e.Type == et {
			return e
		}
	}
	t.Fatalf("Failed to find event with requested type %s", et)
	return nil
}

func collectNotificationsAndEvents(t *testing.T, env *testEnvironment, n int) (httpNotifications []*wallet.TxNotification, wsEvents []*events.NotificationWithSeq) {
	for i := 0; i < n; i++ {
		httpNotifications = append(httpNotifications, env.getNextCallbackNotificationWithTimeout(t))
		wsEvents = append(wsEvents, env.websocketListeners[0].getNextMessageWithTimeout(t))
	}
	return
}

func collectNotifications(t *testing.T, env *testEnvironment, eventType events.EventType, n int) (httpNotifications []*wallet.TxNotification, wsNotifications []*wallet.TxNotification) {
	for i := 0; i < n; i++ {
		httpNotifications = append(httpNotifications, env.getNextCallbackNotificationWithTimeout(t))
		event := env.websocketListeners[0].getNextMessageWithTimeout(t)

		if got, want := event.Type, eventType; got != want {
			t.Fatalf("Unexpected event type for event, wanted %s, got %s:",
				want, got)
		}
		wsNotifications = append(wsNotifications, event.Data.(*wallet.TxNotification))
	}
	return
}
