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
)

func marshalFlattenedEvent(event *NotificationWithSeq) []byte {
	notificationDataJSON, err := json.Marshal(event.Data)
	if err != nil {
		panic(
			fmt.Sprintf(
				"Error: could not json-encode notification for webhook: %v",
				err,
			),
		)

	}

	var flatNotificationData map[string]interface{}
	err = json.Unmarshal(notificationDataJSON, &flatNotificationData)
	if err != nil {
		panic(
			fmt.Sprintf(
				"Error: could not json-decode notificationDataJSON: %v",
				err,
			),
		)
	}

	flatNotificationData["seq"] = event.Seq
	flatNotificationData["type"] = event.Type

	flatNotificationJSON, err := json.Marshal(flatNotificationData)
	if err != nil {
		panic(
			fmt.Sprintf(
				"Error: could not json-encode flat notification: %s",
				err,
			),
		)
	}

	return flatNotificationJSON
}

func (e *eventBroker) sendDataToHTTPCallback(event *NotificationWithSeq) error {
	data := marshalFlattenedEvent(event)
	resp, err := http.Post(
		e.callbackURL,
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
		e.callbackURL,
	)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorText += " also failed to read response body " + err.Error()
	} else {
		errorText += " server replied " + string(body)
	}

	return errors.New(errorText)
}

func (e *eventBroker) handleHTTPCallbackError(event *NotificationWithSeq, err error) bool {
	retry := true
	msg := "Warning: error calling HTTP callback trying to deliver " +
		"event %+v: %s."
	switch {
	case e.httpCallbackBackoff <= 0:
		msg += " Will retry endlessly."
	case e.httpCallbackRetries >= e.httpCallbackBackoff:
		msg += " Retried too many times. Giving up."
		retry = false
	default:
		msg += fmt.Sprintf(
			" Will retry %d more times, next attempt after %f seconds",
			e.httpCallbackBackoff-e.httpCallbackRetries-1,
			e.httpCallbackRetryDelay.Seconds(),
		)
	}

	log.Printf(msg, event, err)

	return retry
}

func (e *eventBroker) sendHTTPCallbackNotifications() {
	seq, err := e.storage.GetLastHTTPSentSeq()
	if err != nil {
		panic(err) // TODO: retry
	}
	events, err := e.GetEventsFromSeq(seq + 1)
	if err != nil {
		panic(err) // TODO: retry
	}
	for _, event := range events {
		if event.Type == NewAddressEvent {
			// NewAddressEvent is not reported via HTTP callback because new
			// addresses are requested via HTTP API - so, caller already knows
			// that address was generated (and which address) from HTTP API
			// response
			continue
		}
		err = e.MakeTransactIfAvailable(func(currBroker *eventBroker) error {
			return currBroker.storage.LockHTTPCallback(event.Seq)
		})
		if err != nil {
			panic(err) // TODO: retry
		}
		err = e.sendDataToHTTPCallback(event)
		e.httpCallbackRetries++
		e.httpCallbackRetryDelay += time.Second
		if err == nil {
			e.httpCallbackRetries = 0
			e.httpCallbackRetryDelay = 0

			err = e.MakeTransactIfAvailable(func(currBroker *eventBroker) error {
				err := currBroker.storage.StoreLastHTTPSentSeq(event.Seq)
				if err != nil {
					return err
				}
				return currBroker.storage.ClearHTTPCallback()
			})
			if err != nil {
				panic(err) // TODO: retry
			}
			continue
		}

		retry := e.handleHTTPCallbackError(event, err)

		if retry {
			err = e.MakeTransactIfAvailable(func(currBroker *eventBroker) error {
				return currBroker.storage.ClearHTTPCallback()
			})
			if err != nil {
				panic(err) // TODO: retry
			}
			time.AfterFunc(
				e.httpCallbackRetryDelay,
				e.triggerHTTPNotificationSending,
			)
			return
		} else {
			err = e.MakeTransactIfAvailable(func(currBroker *eventBroker) error {
				err := currBroker.storage.StoreLastHTTPSentSeq(event.Seq)
				if err != nil {
					return err
				}
				return currBroker.storage.ClearHTTPCallback()
			})
			if err != nil {
				panic(err) // TODO: retry
			}
		}
	}
}
