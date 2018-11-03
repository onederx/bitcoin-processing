package events

import (
	"log"
	"sync"
)

type storedEvent = NotificationWithSeq

type EventStorage interface {
	StoreEvent(event Notification) (*storedEvent, error)
	GetEventsFromSeq(seq int) ([]*storedEvent, error)
}

func newEventStorage(storageType string) EventStorage {
	switch storageType {
	case "memory":
		return &InMemoryEventStorage{
			mutex:  &sync.Mutex{},
			events: make([]*storedEvent, 0),
		}
	case "postgres":
		return newPostgresEventStorage()
	default:
		log.Fatal("Error: unsupported storage type ", storageType)
		return nil
	}
}
