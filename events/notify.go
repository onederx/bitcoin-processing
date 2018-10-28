package events

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/onederx/bitcoin-processing/settings"
)

type EventType string

const (
	EVENT_NEW_ADDRESS     EventType = "new-address"
	EVENT_CHECK_TX_STATUS EventType = "check-tx-status"
)

type notification struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

type notificationWithSeq struct {
	notification
	Seq int `json:"seq"`
}

var EventQueue chan []byte
var ExternalTxNotifications chan string

func init() {
	EventQueue = make(chan []byte)
	ExternalTxNotifications = make(chan string)
}

func notifyHTTPCallback(eventType EventType, data string) {
	callbackUrl := settings.GetURL("tx-callback")
	notificationJSON, err := json.Marshal(notification{eventType, data})
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
	if eventType == EVENT_CHECK_TX_STATUS {
		notifyWalletMayHaveUpdatedWithoutBlocking(data.(string))
		return
	}
	notificationData := notificationWithSeq{
		notification: notification{
			Type: EVENT_NEW_ADDRESS,
			Data: data,
		},
		Seq: 0, // not supported for now
	}
	notificationJSON, err := json.Marshal(notificationData)
	if err != nil {
		log.Printf("Error: could not json-encode notification for ws", err)
		return
	}
	EventQueue <- notificationJSON
}
