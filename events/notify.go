package events

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/settings"
)

type EventType int

const (
	NewAddressEvent EventType = iota
	CheckTxStatusEvent
	NewIncomingTxEvent
	IncomingTxConfirmedEvent
)

func (et EventType) String() string {
	switch et {
	case NewAddressEvent:
		return "new-address"
	case CheckTxStatusEvent:
		return "check-tx-status"
	case NewIncomingTxEvent:
		return "new-incoming-tx"
	case IncomingTxConfirmedEvent:
		return "incoming-tx-confirmed"
	default:
		return "invalid"
	}
}

func (e EventType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + e.String() + "\""), nil
}

type Notification struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

type NotificationWithSeq struct {
	Notification
	Seq int `json:"seq"`
}

var eventBroadcaster *broadcasterWithStorage
var ExternalTxNotifications chan string

func init() {
	ExternalTxNotifications = make(chan string, channelSize)
}

func notifyHTTPCallback(eventType EventType, data string) {
	callbackUrl := settings.GetURL("tx-callback")
	notificationJSON, err := json.Marshal(Notification{eventType, data})
	if err != nil {
		log.Printf("Error: could not json-encode notification for webhook", err)
		return
	}
	resp, err := http.Post(
		callbackUrl,
		"application/json",
		bytes.NewReader(notificationJSON),
	)
	resp.Body.Close()

	if err != nil {
		log.Printf("Error calling webhook", err)
	}
}

func notifyWalletMayHaveUpdatedWithoutBlocking(data string) {
	select {
	case ExternalTxNotifications <- data:
	default:
	}
}

func Notify(eventType EventType, data interface{}) {
	if eventType == CheckTxStatusEvent {
		notifyWalletMayHaveUpdatedWithoutBlocking(data.(string))
		return
	}
	notificationData := storage.StoreEvent(Notification{eventType, data})

	eventBroadcaster.Broadcast(notificationData)
}

func Subscribe() <-chan *NotificationWithSeq {
	return eventBroadcaster.Subscribe()
}

func SubscribeFromSeq(seq int) <-chan *NotificationWithSeq {
	return eventBroadcaster.SubscribeFromSeq(seq)
}

func Unsubscribe(eventChannel <-chan *NotificationWithSeq) {
	eventBroadcaster.Unsubscribe(eventChannel)
}

func Start() {
	initStorage()
}
