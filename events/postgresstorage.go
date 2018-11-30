package events

import (
	"database/sql"
	"encoding/json"
	"log"

	_ "github.com/lib/pq"

	"github.com/onederx/bitcoin-processing/settings"
)

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

	storage := &PostgresEventStorage{
		db: db,
	}
	return storage
}

func (s *PostgresEventStorage) StoreEvent(event Notification) (*storedEvent, error) {
	var seq int
	var err error
	eventDataJSON, err := json.Marshal(&event.Data)
	if err != nil {
		return nil, err
	}
	// XXX: warning: seq can have gaps in case of rollback
	err = s.db.QueryRow(`INSERT INTO events (type, data)
        VALUES ($1, $2) RETURNING seq`, event.Type.String(), eventDataJSON,
	).Scan(&seq)
	if err != nil {
		return nil, err
	}
	return &storedEvent{Notification: event, Seq: seq}, nil
}

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
