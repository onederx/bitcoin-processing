package events

import (
	"sync"

	"github.com/onederx/bitcoin-processing/util"
)

type InMemoryEventStorage struct {
	mutex  *sync.Mutex
	events []*storedEvent
}

func (s *InMemoryEventStorage) StoreEvent(event Notification) (*storedEvent, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	eventWithSeq := &storedEvent{event, len(s.events)}
	s.events = append(s.events, eventWithSeq)

	return eventWithSeq, nil
}

func (s *InMemoryEventStorage) GetEventsFromSeq(seq int) ([]*storedEvent, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.events[util.Min(seq, len(s.events)):], nil
}
