package wallet

import (
	"log"
)

type WalletStorage interface {
	GetLastSeenBlockHash() string
	SetLastSeenBlockHash(blockHash string) error
	StoreTransaction(transaction *Transaction) (*Transaction, error)
	GetTransaction(hash string) (*Transaction, error)
	GetTransactionsWithLessConfirmations(confirmations int64) ([]*Transaction, error)
	updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) error

	GetAccountByAddress(address string) (*Account, error)
	StoreAccount(account *Account) error
}

func newStorage(storageType string) WalletStorage {
	switch storageType {
	case "memory":
		return &InMemoryWalletStorage{
			accounts:     make([]*Account, 0),
			transactions: make([]*Transaction, 0),
		}
	case "postgres":
		return newPostgresWalletStorage()
	default:
		log.Fatal("Error: unsupported storage type ", storageType)
		return nil
	}
}
