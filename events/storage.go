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
	var storage EventStorage

	switch storageType {
	case "memory":
		storage = &InMemoryEventStorage{
			mutex:  &sync.Mutex{},
			events: make([]*storedEvent, 0),
		}
	case "postgres":
		storage = newPostgresEventStorage()
	default:
		log.Fatal("Error: unsupported storage type ", storageType)
	}

	return storage
}
