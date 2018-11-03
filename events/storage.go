package events

import (
	"github.com/onederx/bitcoin-processing/util"
	"log"
	"sync"
)

type storedEvent = NotificationWithSeq

type EventStorage interface {
	StoreEvent(event Notification) *storedEvent
	GetEventsFromSeq(seq int) []*storedEvent
}

type InMemoryEventStorage struct {
	mutex  *sync.Mutex
	events []*storedEvent
}

func (s *InMemoryEventStorage) StoreEvent(event Notification) *storedEvent {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	eventWithSeq := &storedEvent{event, len(s.events)}
	s.events = append(s.events, eventWithSeq)
	return eventWithSeq
}

func (s *InMemoryEventStorage) GetEventsFromSeq(seq int) []*storedEvent {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.events[util.Min(seq, len(s.events)):]
}

func newEventStorage(storageType string) EventStorage {
	if storageType == "memory" {
		return &InMemoryEventStorage{
			mutex:  &sync.Mutex{},
			events: make([]*storedEvent, 0),
		}
	} else {
		log.Fatal("Error: unsupported storage type", storageType)
		return nil
	}
}
