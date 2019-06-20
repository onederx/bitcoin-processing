package events

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofrs/uuid"
	"github.com/lib/pq"

	"github.com/onederx/bitcoin-processing/storage"
)

// PostgresEventStorage stores events in postgresql database. Methods directly
// execute SQL queries that store/fetch required data.
type PostgresEventStorage struct {
	db storage.SQLQueryExecutor
}

func newPostgresEventStorage(db *sql.DB) *PostgresEventStorage {
	return &PostgresEventStorage{db: db}
}

func (s *PostgresEventStorage) getMeta(name string, defaultVal string) (string, error) {
	return storage.GetMeta(s.db, name, defaultVal)
}

func (s *PostgresEventStorage) setMeta(name string, value string) error {
	return storage.SetMeta(s.db, name, value)
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
		LEFT JOIN muted_events
		    ON ((data::jsonb)->>'id')::uuid = muted_events.tx_id
        WHERE seq >= $1 AND muted_events.tx_id is NULL ORDER BY seq`, seq,
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

func (s *PostgresEventStorage) WithTransaction(sqlTX *sql.Tx) EventStorage {
	return &PostgresEventStorage{db: sqlTX}
}

func (s *PostgresEventStorage) GetDB() *sql.DB {
	return s.db.(*sql.DB)
}

func (s *PostgresEventStorage) GetLastHTTPSentSeq() (int, error) {
	lastSeqStr, err := storage.GetMeta(s.db, "last_http_sent_seq", "0")

	if err != nil {
		return 0, err
	}

	return strconv.Atoi(lastSeqStr)
}

func (s *PostgresEventStorage) StoreLastHTTPSentSeq(seq int) error {
	return storage.SetMeta(s.db, "last_http_sent_seq", strconv.Itoa(seq))
}

func (s *PostgresEventStorage) GetLastWSSentSeq() (int, error) {
	lastSeqStr, err := storage.GetMeta(s.db, "last_ws_sent_seq", "0")

	if err != nil {
		return 0, err
	}

	return strconv.Atoi(lastSeqStr)
}

func (s *PostgresEventStorage) StoreLastWSSentSeq(seq int) error {
	return storage.SetMeta(s.db, "last_ws_sent_seq", strconv.Itoa(seq))
}

func (s *PostgresEventStorage) LockHTTPCallback(operation interface{}) error {
	operationMarshaled, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	return s.setMeta("http_callback_operation", string(operationMarshaled))
}

func (s *PostgresEventStorage) ClearHTTPCallback() error {
	return s.setMeta("http_callback_operation", "")
}

func (s *PostgresEventStorage) CheckHTTPCallbackLock() (bool, string, error) {
	operation, err := s.getMeta("http_callback_operation", "")
	return operation == "", operation, err
}

func (s *PostgresEventStorage) GetNextEventFromSeq(seq int) (*storedEvent, error) {
	var (
		eventSeq                    int
		eventTypeStr, marshaledData string
		event                       storedEvent
	)

	row := s.db.QueryRow(
		`SELECT seq, type, data FROM events
		LEFT JOIN muted_events
		    ON ((data::jsonb)->>'id')::uuid = muted_events.tx_id
        WHERE seq >= $1 AND muted_events.tx_id is NULL
        ORDER BY seq
        LIMIT 1`, seq)

	err := row.Scan(&eventSeq, &eventTypeStr, &marshaledData)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoSuchEvent
		}
		return nil, err
	}

	notificationJSON := fmt.Sprintf(`{"seq": %d, "type": "%s", "data": %s}`,
		eventSeq, eventTypeStr, marshaledData)
	fmt.Printf("%s\n", notificationJSON)

	err = json.Unmarshal([]byte(notificationJSON), &event)

	if err != nil {
		return nil, err
	}

	return &event, err
}

func (s *PostgresEventStorage) MuteEventsWithTxID(id uuid.UUID) error {
	const uniqConstrViolated = pq.ErrorCode("23505")

	_, err := s.db.Exec(`INSERT INTO muted_events (tx_id) values ($1)`, id)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == uniqConstrViolated {
			return ErrAlreadyMuted
		}
	}

	return err
}
