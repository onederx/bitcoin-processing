package wallet

import (
	"log"

	"github.com/satori/go.uuid"
)

type Storage interface {
	GetLastSeenBlockHash() string
	SetLastSeenBlockHash(blockHash string) error
	StoreTransaction(transaction *Transaction) (*Transaction, error)
	GetTransactionByHash(hash string) (*Transaction, error)
	GetTransactionByID(id uuid.UUID) (*Transaction, error)
	GetBroadcastedTransactionsWithLessConfirmations(confirmations int64) ([]*Transaction, error)
	GetPendingTransactions() ([]*Transaction, error)
	updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) error
	GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*Transaction, error)

	GetAccountByAddress(address string) (*Account, error)
	StoreAccount(account *Account) error

	GetHotWalletAddress() string
	SetHotWalletAddress(address string) error

	GetMoneyRequiredFromColdStorage() uint64
	SetMoneyRequiredFromColdStorage(amount uint64) error
}

func newStorage(storageType string) Storage {
	var storage Storage

	switch storageType {
	case "memory":
		storage = &InMemoryWalletStorage{
			accounts:     make([]*Account, 0),
			transactions: make([]*Transaction, 0),
		}
	case "postgres":
		storage = newPostgresWalletStorage()
	default:
		log.Fatal("Error: unsupported storage type ", storageType)
	}

	return storage
}
