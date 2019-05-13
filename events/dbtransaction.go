package events

import (
	"database/sql"

	"github.com/onederx/bitcoin-processing/storage"
)

func (e *eventBroker) withTransaction(sqlTX *sql.Tx) *eventBroker {
	return &eventBroker{
		eventBrokerData: e.eventBrokerData,
		storage:         e.storage.WithTransaction(sqlTX),
	}
}

func (e *eventBroker) WithTransaction(sqlTX *sql.Tx) EventBroker {
	return e.withTransaction(sqlTX)
}

func (e *eventBroker) MakeTransactIfAvailable(f func(*eventBroker) error) error {
	return storage.MakeTransactIfAvailable(e.database, func(sqlTx *sql.Tx) error {
		broker := e
		if sqlTx != nil {
			broker = e.withTransaction(sqlTx)
		}
		return f(broker)
	})
}
