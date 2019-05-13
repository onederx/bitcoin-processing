package storage

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq" // Enable postgresql driver

	"github.com/onederx/bitcoin-processing/settings"
)

type SQLQueryExecutor interface {
	Query(string, ...interface{}) (*sql.Rows, error)
	Exec(string, ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func Open(s settings.Settings) *sql.DB {
	var (
		db  *sql.DB
		err error
	)

	storageType := s.GetStringMandatory("storage.type")

	switch storageType {
	case "memory":
		db = nil
	case "postgres":
		dsn := s.GetStringMandatory("storage.dsn")

		db, err = sql.Open("postgres", dsn)

		if err != nil {
			panic(err)
		}

		err = db.Ping()
		if err != nil {
			panic(err)
		}
	default:
		panic("Error: unsupported storage type " + storageType)
	}

	return db
}

func GetMeta(e SQLQueryExecutor, name string, defaultVal string) (string, error) {
	result := defaultVal
	err := e.QueryRow(`SELECT value FROM metadata WHERE key = $1`, name).Scan(&result)
	if err == nil || err == sql.ErrNoRows {
		return result, nil
	}
	return "", err
}

func SetMeta(e SQLQueryExecutor, name string, value string) error {
	_, err := e.Exec(`INSERT INTO metadata (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, name, value)
	return err
}

func MakeTransactIfAvailable(db *sql.DB, f func(tx *sql.Tx) error) error {
	var (
		tx  *sql.Tx
		err error
	)

	commited := false

	if db != nil {
		tx, err = db.Begin()
		if err != nil {
			return err
		}
		defer func() {
			if !commited {
				err := tx.Rollback()
				if err != nil {
					// XXX: panic here?
					// We will probably get here in case of db connection loss
					log.Printf(
						"CRITICAL: storage: failed to roll back tx: %v", err,
					)
				}
			}
		}()
	}

	err = f(tx)

	if err != nil {
		return err
	}

	if tx != nil {
		err = tx.Commit()
		commited = true
	}

	return err
}
