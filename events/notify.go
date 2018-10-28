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
	EVENT_NEW_ADDRESS EventType = "new-address"
)

type notification struct {
	Type EventType `json:"type"`
	Data string    `json:"data"`
}

type notificationWithSeq struct {
	notification
	Seq int `json:"seq"`
}

var EventQueue chan []byte

func init() {
	EventQueue = make(chan []byte)
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

func Notify(eventType EventType, data string) {
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
