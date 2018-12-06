package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/onederx/bitcoin-processing/settings"
)

func (e *EventBroker) notifyHTTPCallback(event *NotificationWithSeq) {
	notificationDataJSON, err := json.Marshal(event.Data)
	if err != nil {
		log.Printf("Error: could not json-encode notification for webhook: %s", err)
		return
	}

	var flatNotificationData map[string]interface{}
	err = json.Unmarshal(notificationDataJSON, &flatNotificationData)
	if err != nil {
		log.Printf("Error: could not json-decode notificationDataJSON: %s", err)
		return
	}

	flatNotificationData["seq"] = event.Seq
	flatNotificationData["type"] = event.Type

	flatNotificationJSON, err := json.Marshal(flatNotificationData)
	if err != nil {
		log.Printf("Error: could not json-encode flat notification: %s", err)
		return
	}

	select {
	case e.callbackUrlQueue <- flatNotificationJSON:
	default:
	}
}

func (e *EventBroker) sendDataToHTTPCallback(data []byte) error {
	resp, err := http.Post(
		e.callbackUrl,
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	errorText := fmt.Sprintf(
		"Got response with code %d calling HTTP callback %s",
		resp.StatusCode,
		e.callbackUrl,
	)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorText += " also failed to read response body " + err.Error()
	} else {
		errorText += " server replied " + string(body)
	}

	return errors.New(errorText)
}

func (e *EventBroker) sendHTTPCallbackNotifications() {
	backoff := settings.GetInt("transaction.callback.backoff")

	for notification := range e.callbackUrlQueue {
		delay := time.Second
		for attempts := 0; backoff <= 0 || attempts < backoff; attempts++ {
			err := e.sendDataToHTTPCallback(notification)
			if err == nil {
				break
			}
			msg := "Warning: error calling HTTP callback: %s."
			switch {
			case backoff <= 0:
				msg += " Will retry endlessly."
			case attempts >= backoff-1:
				msg += " Retried too many times. Giving up."
			default:
				msg += fmt.Sprintf(
					" Will retry %d more times, next attempt after %f seconds",
					backoff-attempts-1,
					delay.Seconds(),
				)
			}
			log.Printf(msg, err)
			time.Sleep(delay)
			delay += time.Second
		}
	}
}
