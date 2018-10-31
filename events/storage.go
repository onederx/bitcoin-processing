package events

import (
	"github.com/onederx/bitcoin-processing/settings"
	"log"
	"sync"
)

type storedEvent = NotificationWithSeq

type EventStorage interface {
	StoreEvent(event Notification) *storedEvent
	GetEventsFromSeq(seq int) []*storedEvent
}

var storage EventStorage

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
	return s.events[seq:]
}

func initStorage() {
	storageType := settings.GetStringMandatory("storage.type")

	if storageType == "memory" {
		storage = &InMemoryEventStorage{
			mutex:  &sync.Mutex{},
			events: make([]*storedEvent, 0),
		}
	} else {
		log.Fatal("Error: unsupported storage type", storageType)
	}
}
