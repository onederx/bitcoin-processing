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

const (
	httpCBResultSaveRetries           = 10
	httpCBResultSaveRetryBaseInterval = time.Second
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
		log.Printf("Successfully sent HTTP notification with seq %d.", event.Seq)
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

func (e *eventBroker) handleEarlyErrorPreparingHTTPCallback(err error, action string) {
	// early error means we haven't sent anything yet, so we can survive this
	// error and try again later
	log.Printf("events: http callback sender: got error %s: %v", action, err)
	log.Printf("will retry after a delay")
	time.AfterFunc(
		e.httpCallbackRetryDelay,
		e.triggerHTTPNotificationSending,
	)
}

func (e *eventBroker) sendHTTPCallbackNotifications(isRetry bool) {
	if e.httpCallbackIsRetrying {
		if !isRetry {
			// avoid retries before interval elapses
			return
		}
		e.httpCallbackIsRetrying = false
	}

	seq, err := e.storage.GetLastHTTPSentSeq()
	if err != nil {
		e.handleEarlyErrorPreparingHTTPCallback(err, "getting last sent seqnum")
		return
	}
	events, err := e.GetEventsFromSeq(seq + 1)
	if err != nil {
		e.handleEarlyErrorPreparingHTTPCallback(err, "getting events from storage")
		return
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
			e.handleEarlyErrorPreparingHTTPCallback(
				err,
				"setting dirty bit for HTTP callback in DB",
			)
			return
		}
		err = e.sendDataToHTTPCallback(event)
		e.httpCallbackRetries++
		e.httpCallbackRetryDelay += time.Second

		if err != nil {
			retry := e.handleHTTPCallbackError(event, err)
			if retry {
				persistHTTPCallbackSendResultWithRetry(func() error {
					return e.MakeTransactIfAvailable(func(currBroker *eventBroker) error {
						return currBroker.storage.ClearHTTPCallback()
					})
				}, false)
				e.httpCallbackIsRetrying = true
				time.AfterFunc(
					e.httpCallbackRetryDelay,
					e.triggerHTTPNotificationRetry,
				)
				return
			}
		}

		// if we're here, either sending callback was OK, or we have given up
		// retrying to send notification
		// in both cases, we want to stop retrying and pass on to next event

		persistHTTPCallbackSendResultWithRetry(func() error {
			return e.MakeTransactIfAvailable(func(currBroker *eventBroker) error {
				err := currBroker.storage.StoreLastHTTPSentSeq(event.Seq)
				if err != nil {
					return err
				}
				return currBroker.storage.ClearHTTPCallback()
			})
		}, true)
		e.httpCallbackRetries = 0
		e.httpCallbackRetryDelay = 0
	}
}

func persistHTTPCallbackSendResultWithRetry(persistFunc func() error, success bool) {
	retries := httpCBResultSaveRetries
	interval := httpCBResultSaveRetryBaseInterval

	for {
		err := persistFunc()

		if err == nil {
			if retries < httpCBResultSaveRetries {
				log.Printf(
					"Succeeded to save http cb send result after %d attempts",
					(httpCBResultSaveRetries-retries)+1,
				)
			}
			return
		}

		log.Printf(
			"events: CRITICAL: failed to save http callback send result to "+
				"DB: %v", err,
		)

		if success {
			log.Printf("(result being saved is success)")
		} else {
			log.Printf("(result being saved is failure)")
		}

		if retries > 0 {
			log.Printf("Will retry after %s, %d retries left", interval, retries)
		} else {
			log.Panic("FATAL: given up, aborting event broker")
		}
		time.Sleep(interval)
		retries--
		interval += httpCBResultSaveRetryBaseInterval
	}
}
