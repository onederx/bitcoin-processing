package wallet

import (
	"database/sql"

	"github.com/onederx/bitcoin-processing/storage"
)

func (w *Wallet) WithTransaction(sqlTX *sql.Tx) *Wallet {
	return &Wallet{
		walletData:  w.walletData,
		storage:     w.storage.WithTransaction(sqlTX),
		eventBroker: w.eventBroker.WithTransaction(sqlTX),
	}
}

func (w *Wallet) MakeTransactIfAvailable(f func(*Wallet) error) error {
	return storage.MakeTransactIfAvailable(w.database, func(sqlTx *sql.Tx) error {
		wallet := w
		if sqlTx != nil {
			wallet = w.WithTransaction(sqlTx)
		}
		return f(wallet)
	})
}
