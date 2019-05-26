package events

import (
	"database/sql"
	"time"

	"github.com/onederx/bitcoin-processing/settings"
)

type eventBrokerData struct {
	eventBroadcaster *broadcasterWithStorage
	database         *sql.DB

	// TODO: move this comment to wallet
	// Channel ExternalTxNotifications can be used to notify wallet that there
	// are relevant tx updates and that it should get updates from Bitcoin node
	// immediately (without it, updates are polled periodically). Wallet updater
	// reads this channel to get notification

	callbackURL            string
	httpCallbackBackoff    int
	httpCallbackRetries    int
	httpCallbackRetryDelay time.Duration

	wsNotificationTrigger           chan struct{}
	httpCallbackNotificationTrigger chan bool
	httpCallbackIsRetrying          bool
}

// eventBroker is responsible for processing events - sending them to client
// via http callback and websocket and storing them in DB
type eventBroker struct {
	*eventBrokerData
	storage EventStorage
}

// NewEventBroker creates new instance of eventBroker
func NewEventBroker(s settings.Settings, storage EventStorage) EventBroker {
	return &eventBroker{
		storage: storage,
		eventBrokerData: &eventBrokerData{
			eventBroadcaster:                newBroadcasterWithStorage(storage),
			database:                        storage.GetDB(),
			callbackURL:                     s.GetURL("transaction.callback.url"),
			httpCallbackBackoff:             s.GetInt("transaction.callback.backoff"),
			wsNotificationTrigger:           make(chan struct{}, 3),
			httpCallbackNotificationTrigger: make(chan bool, 3),
		},
	}
}

// Notify creates new event with given type and associated data. The event will
// be processed depending on its type.
func (e *eventBroker) Notify(eventType EventType, data interface{}) error {
	_, err := e.storage.StoreEvent(Notification{eventType, data})
	return err
}

func (e *eventBroker) SendNotifications() {
	e.triggerWSNotificationSending()
	e.triggerHTTPNotificationSending()
}

func (e *eventBroker) triggerWSNotificationSending() {
	select {
	case e.wsNotificationTrigger <- struct{}{}:
	default:
	}
}

func (e *eventBroker) triggerHTTPNotificationSending() {
	select {
	case e.httpCallbackNotificationTrigger <- false:
	default:
	}
}

func (e *eventBroker) triggerHTTPNotificationRetry() {
	// NB: blocking send so that retry is not lost
	e.httpCallbackNotificationTrigger <- true
}

// SubscribeFromSeq allows to get old events starting with given sequence number
// and new ones. It returns a channel of SLICES of events. When loading old
// events from DB, all events that were fetched simultaneously will be written
// to a channel in one slice, not one by one. Otherwise, with large number of
// events channel may overflow. Subscriber should iterate over each slice to
// get all events.
// This method is used for websocket subscription
func (e *eventBroker) SubscribeFromSeq(seq int) chan []*NotificationWithSeq {
	subch := e.eventBroadcaster.SubscribeFromSeq(seq)
	e.triggerWSNotificationSending()
	return subch
}

// UnsubscribeFromSeq cancels subscription created by SubscribeFromSeq. Channel
// given to it as an argument must be one returned by SubscribeFromSeq
func (e *eventBroker) UnsubscribeFromSeq(eventChannel chan []*NotificationWithSeq) {
	e.eventBroadcaster.Unsubscribe(eventChannel)
}

// GetEventsFromSeq returns a slice of old events starting with given sequence
// number. This method is used by HTTP API endpoint /get_events
func (e *eventBroker) GetEventsFromSeq(seq int) ([]*NotificationWithSeq, error) {
	return e.storage.GetEventsFromSeq(seq)
}

// TODO: move this comment to wallet or delete it
// GetExternalTxNotificationChannel returns a channel that can be used to
// subscribe on events that Bitcoin node should be checked for updates because
// there are new relevant txns or there are updates on existing ones.
// For every new CheckTxStatusEvent an element will be sent to this channel
// (an element is it's data argument which should be a tx id)

func (e *eventBroker) sendNotifications() {
	for {
		select {
		case <-e.wsNotificationTrigger:
			e.sendWSNotifications()
		case isRetry := <-e.httpCallbackNotificationTrigger:
			e.sendHTTPCallbackNotifications(isRetry)
		}
	}
}

// Run starts event broker. Most of the broker is event-driven, routine started
// by Run is http callback notifier, which works in background because it has
// to retry requests with backoff and maintains a queue of requests
func (e *eventBroker) Run() {
	e.sendNotifications()
}
