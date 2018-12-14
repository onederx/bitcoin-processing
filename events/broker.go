package events

import (
	"log"

	"github.com/onederx/bitcoin-processing/settings"
)

const callbackUrlQueueSize = 100000

type EventBroker struct {
	storage                 EventStorage
	eventBroadcaster        *broadcasterWithStorage
	ExternalTxNotifications chan string
	callbackUrl             string
	callbackUrlQueue        chan []byte
}

func NewEventBroker() *EventBroker {
	storageType := settings.GetStringMandatory("storage.type")
	storage := newEventStorage(storageType)
	return &EventBroker{
		storage:                 storage,
		eventBroadcaster:        newBroadcasterWithStorage(storage),
		ExternalTxNotifications: make(chan string, 3),
		callbackUrl:             settings.GetURL("transaction.callback.url"),
		callbackUrlQueue:        make(chan []byte, callbackUrlQueueSize),
	}
}

func (e *EventBroker) notifyWalletMayHaveUpdatedWithoutBlocking(data string) {
	select {
	case e.ExternalTxNotifications <- data:
	default:
	}
}

func (e *EventBroker) Notify(eventType EventType, data interface{}) {
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

func (e *EventBroker) Subscribe() <-chan *NotificationWithSeq {
	return e.eventBroadcaster.Subscribe()
}

func (e *EventBroker) SubscribeFromSeq(seq int) <-chan []*NotificationWithSeq {
	return e.eventBroadcaster.SubscribeFromSeq(seq)
}

func (e *EventBroker) UnsubscribeFromSeq(eventChannel <-chan []*NotificationWithSeq) {
	e.eventBroadcaster.UnsubscribeFromSeq(eventChannel)
}

func (e *EventBroker) GetEventsFromSeq(seq int) ([]*NotificationWithSeq, error) {
	return e.storage.GetEventsFromSeq(seq)
}

func (e *EventBroker) Run() {
	e.sendHTTPCallbackNotifications()
}
