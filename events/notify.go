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
	NewAddressEvent          EventType = "new-address"
	CheckTxStatusEvent                 = "check-tx-status"
	NewIncomingTxEvent                 = "new-incoming-tx"
	IncomingTxConfirmedEvent           = "incoming-tx-confirmed"
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
	if eventType == CheckTxStatusEvent {
		notifyWalletMayHaveUpdatedWithoutBlocking(data.(string))
		return
	}
	notificationData := storage.StoreEvent(notification{eventType, data})

	notificationJSON, err := json.Marshal(notificationData)
	if err != nil {
		log.Printf("Error: could not json-encode notification for ws", err)
		return
	}
	EventQueue <- notificationJSON
}

func Start() {
	initStorage()
}
