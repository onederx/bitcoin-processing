package events

import (
	"database/sql"
	"encoding/json"
	"log"

	_ "github.com/lib/pq" // Enable postgresql driver

	"github.com/onederx/bitcoin-processing/settings"
)

// PostgresEventStorage stores events in postgresql database. Methods directly
// execute SQL queries that store/fetch required data.
type PostgresEventStorage struct {
	db *sql.DB
}

func newPostgresEventStorage() *PostgresEventStorage {
	dsn := settings.GetStringMandatory("storage.dsn")

	db, err := sql.Open("postgres", dsn)

	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	return &PostgresEventStorage{db: db}
}

// StoreEvent stores event in database, assigning a sequence number to it.
// Retuned storedEvent has the same type and data as event arg, but also has a
// sequence number.
// XXX: warning: sequence numbers can have gaps in case of rollback. Sequence
// number is an auto-incrementing private key of type SERIAL in table with
// events and, if transaction incremented it and then was rolled back, it won't
// decrement back creating a hole.
func (s *PostgresEventStorage) StoreEvent(event Notification) (*storedEvent, error) {
	eventDataJSON, err := json.Marshal(&event.Data)
	if err != nil {
		return nil, err
	}

	var seq int
	err = s.db.QueryRow(`INSERT INTO events (type, data)
        VALUES ($1, $2) RETURNING seq`, event.Type.String(), eventDataJSON,
	).Scan(&seq)
	if err != nil {
		return nil, err
	}

	return &storedEvent{Notification: event, Seq: seq}, nil
}

// GetEventsFromSeq fetches events from DB starting with given sequence number
// and returns them as a slice.
func (s *PostgresEventStorage) GetEventsFromSeq(seq int) ([]*storedEvent, error) {
	var eventSeq int
	var eventTypeStr, marshaledData string
	var eventData interface{}

	result := make([]*storedEvent, 0, 20)

	rows, err := s.db.Query(`SELECT seq, type, data FROM events
        WHERE seq >= $1 ORDER BY seq`, seq,
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&eventSeq, &eventTypeStr, &marshaledData)
		if err != nil {
			return result, err
		}
		eventType, err := EventTypeFromString(eventTypeStr)
		if err != nil {
			return result, err
		}
		err = json.Unmarshal([]byte(marshaledData), &eventData)

		if err != nil {
			return result, err
		}
		result = append(result, &storedEvent{
			Notification: Notification{
				Type: eventType,
				Data: eventData,
			},
			Seq: eventSeq,
		})
	}
	err = rows.Err()
	if err != nil {
		return result, err
	}

	return result, nil
}
