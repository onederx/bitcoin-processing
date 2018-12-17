package events

import (
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
