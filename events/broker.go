package events

import (
	"log"

	"github.com/onederx/bitcoin-processing/settings"
)

const callbackURLQueueSize = 100000

// eventBroker is responsible for processing events - sending them to client
// via http callback and websocket and storing them in DB
type eventBroker struct {
	storage          EventStorage
	eventBroadcaster *broadcasterWithStorage

	// Channel ExternalTxNotifications can be used to notify wallet that there
	// are relevant tx updates and that it should get updates from Bitcoin node
	// immediately (without it, updates are polled periodically). Wallet updater
	// reads this channel to get notification
	ExternalTxNotifications chan string

	callbackURL         string
	callbackURLQueue    chan []byte
	httpCallbackBackoff int
}

// NewEventBroker creates new instance of eventBroker
func NewEventBroker(s settings.Settings) EventBroker {
	storageType := s.GetStringMandatory("storage.type")
	storage := newEventStorage(storageType, s)
	return &eventBroker{
		storage:                 storage,
		eventBroadcaster:        newBroadcasterWithStorage(storage),
		ExternalTxNotifications: make(chan string, 3),
		callbackURL:             s.GetURL("transaction.callback.url"),
		callbackURLQueue:        make(chan []byte, callbackURLQueueSize),
		httpCallbackBackoff:     s.GetInt("transaction.callback.backoff"),
	}
}

func (e *eventBroker) notifyWalletMayHaveUpdatedWithoutBlocking(data string) {
	select {
	case e.ExternalTxNotifications <- data:
	default:
	}
}

// Notify creates new event with given type and associated data. The event will
// be processed depending on its type.
func (e *eventBroker) Notify(eventType EventType, data interface{}) {
	if eventType == CheckTxStatusEvent {
		e.notifyWalletMayHaveUpdatedWithoutBlocking(data.(string))
		return
	}

	notificationData, err := e.storage.StoreEvent(Notification{eventType, data})
	if err != nil {
		log.Printf(
			"Error: failed to store event type %s with data %v: %s",
			eventType.String(),
			data,
			err,
		)
		return // TODO: do not send event to subscribers? Maybe send with seq = -1 ?
	}

	e.eventBroadcaster.Broadcast(notificationData)

	if eventType != NewAddressEvent {
		e.notifyHTTPCallback(notificationData)
	}
}

// SubscribeFromSeq allows to get old events starting with given sequence number
// and new ones. It returns a channel of SLICES of events. When loading old
// events from DB, all events that were fetched simultaneously will be written
// to a channel in one slice, not one by one. Otherwise, with large number of
// events channel may overflow. Subscriber should iterate over each slice to
// get all events.
// This method is used for websocket subscription
func (e *eventBroker) SubscribeFromSeq(seq int) <-chan []*NotificationWithSeq {
	return e.eventBroadcaster.SubscribeFromSeq(seq)
}

// UnsubscribeFromSeq cancels subscription created by SubscribeFromSeq. Channel
// given to it as an argument must be one returned by SubscribeFromSeq
func (e *eventBroker) UnsubscribeFromSeq(eventChannel <-chan []*NotificationWithSeq) {
	e.eventBroadcaster.UnsubscribeFromSeq(eventChannel)
}

// GetEventsFromSeq returns a slice of old events starting with given sequence
// number. This method is used by HTTP API endpoint /get_events
func (e *eventBroker) GetEventsFromSeq(seq int) ([]*NotificationWithSeq, error) {
	return e.storage.GetEventsFromSeq(seq)
}

// GetExternalTxNotificationChannel returns a channel that can be used to
// subscribe on events that Bitcoin node should be checked for updates because
// there are new relevant txns or there are updates on existing ones.
// For every new CheckTxStatusEvent an element will be sent to this channel
// (an element is it's data argument which should be a tx id)
func (e *eventBroker) GetExternalTxNotificationChannel() chan string {
	return e.ExternalTxNotifications
}

// Run starts event broker. Most of the broker is event-driven, routine started
// by Run is http callback notifier, which works in background because it has
// to retry requests with backoff and maintains a queue of requests
func (e *eventBroker) Run() {
	e.sendHTTPCallbackNotifications()
}
