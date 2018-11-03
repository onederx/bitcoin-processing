package events

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/settings"
)

type EventBroker struct {
	storage                 EventStorage
	eventBroadcaster        *broadcasterWithStorage
	ExternalTxNotifications chan string
	callbackUrl             string
}

func NewEventBroker() *EventBroker {
	storageType := settings.GetStringMandatory("storage.type")
	storage := newEventStorage(storageType)
	return &EventBroker{
		storage:                 storage,
		eventBroadcaster:        newBroadcasterWithStorage(storage),
		ExternalTxNotifications: make(chan string, 3),
		callbackUrl:             settings.GetURL("transaction.callback"),
	}
}

func (e *EventBroker) notifyHTTPCallback(event *NotificationWithSeq) {
	notificationJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error: could not json-encode notification for webhook", err)
		return
	}
	resp, err := http.Post(
		e.callbackUrl,
		"application/json",
		bytes.NewReader(notificationJSON),
	)
	if err != nil {
		log.Print("Warning: error calling webhook ", err)
		return
	}
	resp.Body.Close()
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

func (e *EventBroker) SubscribeFromSeq(seq int) <-chan *NotificationWithSeq {
	return e.eventBroadcaster.SubscribeFromSeq(seq)
}

func (e *EventBroker) Unsubscribe(eventChannel <-chan *NotificationWithSeq) {
	e.eventBroadcaster.Unsubscribe(eventChannel)
}
