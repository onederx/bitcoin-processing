package events

import (
	"errors"
	"log"

	"github.com/gofrs/uuid"

	wallettypes "github.com/onederx/bitcoin-processing/wallet/types"
)

type internalMuteRequest struct {
	txID   uuid.UUID
	result chan error
}

func (e *eventBroker) mute(txID uuid.UUID) error {
	if txID == uuid.Nil {
		if !e.httpCallbackIsRetrying || e.httpCallbackRetryingSeq == -1 {
			return errors.New("Event broker is not currently retrying to " +
				"deliver an event")
		}
		event, err := e.storage.GetNextEventFromSeq(e.httpCallbackRetryingSeq)

		if err != nil {
			if err == ErrNoSuchEvent {
				return errors.New("Failed to find an event to mute")
			}
			return err
		}
		if notification, ok := event.Data.(*wallettypes.TxNotification); ok {
			txID = notification.ID
			log.Printf("Muting tx id %s for currently retried event", txID)
		} else {
			return errors.New(
				"Current event being retried does not refer to a tx",
			)
		}
	}

	log.Printf("Mute events for tx id %s", txID)

	err := e.storage.MuteEventsWithTxID(txID)

	if err != nil {
		log.Printf("Mute failed, err: %v", err)
		return err
	}

	log.Printf("Muted OK")

	if e.httpCallbackIsRetrying {
		log.Printf("Successfully muted an event, resetting http cb retry state")
		e.httpCallbackIsRetrying = true
		e.httpCallbackRetries = 0
		e.httpCallbackRetryDelay = 0
	}
	return nil
}

func (e *eventBroker) MuteEventsWithTxID(txID uuid.UUID) error {
	resultCh := make(chan error)
	e.muteRequests <- internalMuteRequest{
		txID:   txID,
		result: resultCh,
	}
	return <-resultCh

}
