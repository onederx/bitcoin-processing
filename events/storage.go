package events

import (
	"database/sql"
	"log"
	"sync"
)

type storedEvent = NotificationWithSeq

// EventStorage is an interface that should be implemented by types that store
// events for EventBroker.
// Storage supports an ability of broker clients to get a view of past events
// in case they missed them by assigning sequence numbers to events and allowing
// to get events starting from some sequence number.
// Two methods are required by this interface: StoreEvent that adds new event to
// storage assigning a sequence number to it and GetEventsFromSeq that fetches a
// slice of events from storage which have sequence number greater or equal to
// given one
type EventStorage interface {
	// StoreEvent takes Notification (which has type and data, but not sequence
	// number), stores it and assigns a sequence number to it. It returns a
	// pointer to storedEvent (which is an alias to NotificationWithSeq),
	// which has same type and data, but also now has a sequence number.
	StoreEvent(event Notification) (*storedEvent, error)

	// GetEventsFromSeq returns a slice of events that have sequence number
	// greater or equal to given value and are sorted by sequence numbers in
	// ascending order. In case given sequece number is larger than maximum
	// sequence number in storage, empty slice should be returned.
	GetEventsFromSeq(seq int) ([]*storedEvent, error)

	GetLastHTTPSentSeq() (int, error)
	StoreLastHTTPSentSeq(seq int) error

	LockHTTPCallback(operation interface{}) error
	ClearHTTPCallback() error
	CheckHTTPCallbackLock() (bool, string, error)

	WithTransaction(sqlTX *sql.Tx) EventStorage
	GetDB() *sql.DB
}

func NewEventStorage(db *sql.DB) EventStorage {
	if db == nil {
		log.Print("Warning: initializing in-memory event storage since no db " +
			"connection is passed. Note it should not be used in production")
		return &InMemoryEventStorage{
			mutex:  &sync.Mutex{},
			events: make([]*storedEvent, 0),
		}
	}
	return newPostgresEventStorage(db)
}
