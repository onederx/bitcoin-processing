package events

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"

	"github.com/onederx/bitcoin-processing/util"
)

// InMemoryEventStorage stores events in memory, simply using a slice of
// pointers. It implements EventStorage interface. InMemoryEventStorage is
// made for testing only (to get a working EventStorage implementation without
// external dependencies) and does not provide any kind of persistence, safety
// or efficiency. PostgresEventStorage should be used in production.
//
// In future InMemoryEventStorage may be used as a cache in front of
// PostgresEventStorage to make storage faster, but it is not implemented now
type InMemoryEventStorage struct {
	mutex  *sync.Mutex
	events []*storedEvent

	lastHTTPSentSeq int
	lastWSSentSeq   int

	httpCallbackOperation string
	wsOperation           string
}

// StoreEvent adds event to storage. Implementation is naive: it actually just
// appends event to a slice and index in that slice becomes its sequence number.
// Retuned storedEvent has the same type and data as event arg, but also has a
// sequence number.
func (s *InMemoryEventStorage) StoreEvent(event Notification) (*storedEvent, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	eventWithSeq := &storedEvent{event, len(s.events)}
	s.events = append(s.events, eventWithSeq)

	return eventWithSeq, nil
}

// GetEventsFromSeq returns events from storage with sequence number greater or
// equal to given one. It returns a subslice of internal storage, so
// modifications of events returned will change events in storage.
func (s *InMemoryEventStorage) GetEventsFromSeq(seq int) ([]*storedEvent, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.events[util.Min(seq, len(s.events)):], nil
}

func (s *InMemoryEventStorage) WithTransaction(sqlTX *sql.Tx) EventStorage {
	log.Printf(
		"Warning: WithTransaction called on memory event storage. Memory " +
			"storage does not support transactions, so it just does nothing.",
	)
	return s
}

func (s *InMemoryEventStorage) GetDB() *sql.DB {
	return nil
}

func (s *InMemoryEventStorage) GetLastHTTPSentSeq() (int, error) {
	return s.lastHTTPSentSeq, nil
}

func (s *InMemoryEventStorage) StoreLastHTTPSentSeq(seq int) error {
	s.lastHTTPSentSeq = seq
	return nil
}

func (s *InMemoryEventStorage) GetLastWSSentSeq() (int, error) {
	return s.lastWSSentSeq, nil
}

func (s *InMemoryEventStorage) StoreLastWSSentSeq(seq int) error {
	s.lastWSSentSeq = seq
	return nil
}

func (s *InMemoryEventStorage) LockHTTPCallback(operation interface{}) error {
	operationMarshaled, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	s.httpCallbackOperation = string(operationMarshaled)
	return nil
}

func (s *InMemoryEventStorage) ClearHTTPCallback() error {
	s.httpCallbackOperation = ""
	return nil
}

func (s *InMemoryEventStorage) CheckHTTPCallbackLock() (bool, string, error) {
	return s.httpCallbackOperation == "", s.httpCallbackOperation, nil
}

func (s *InMemoryEventStorage) LockWS(operation interface{}) error {
	operationMarshaled, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	s.wsOperation = string(operationMarshaled)
	return nil
}

func (s *InMemoryEventStorage) ClearWS() error {
	s.wsOperation = ""
	return nil
}

func (s *InMemoryEventStorage) CheckWSLock() (bool, string, error) {
	return s.wsOperation == "", s.wsOperation, nil
}
