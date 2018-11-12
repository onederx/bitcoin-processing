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
	notificationJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error: could not json-encode notification for webhook", err)
		return
	}
	select {
	case e.callbackUrlQueue <- notificationJSON:
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
	if resp.StatusCode != 200 {
		errorText := fmt.Sprintf(
			"Got response with code %d calling HTTP callback %s",
			resp.StatusCode,
			e.callbackUrl,
		)
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			return errors.New(errorText + " server replied " + string(body))
		} else {
			return errors.New(
				errorText + " also failed to read response body " + err.Error(),
			)
		}
	}
	return nil
}

func (e *EventBroker) sendHTTPCallbackNotifications() {
	backoff := settings.GetInt("callback.backoff")

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
