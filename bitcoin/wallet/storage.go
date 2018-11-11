package wallet

import (
	"log"

	"github.com/satori/go.uuid"
)

type WalletStorage interface {
	GetLastSeenBlockHash() string
	SetLastSeenBlockHash(blockHash string) error
	StoreTransaction(transaction *Transaction) (*Transaction, error)
	GetTransactionByHash(hash string) (*Transaction, error)
	GetTransactionById(id uuid.UUID) (*Transaction, error)
	GetBroadcastedTransactionsWithLessConfirmations(confirmations int64) ([]*Transaction, error)
	GetPendingTransactions() ([]*Transaction, error)
	updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) error

	GetAccountByAddress(address string) (*Account, error)
	StoreAccount(account *Account) error

	GetHotWalletAddress() string
	SetHotWalletAddress(address string) error

	GetMoneyRequiredFromColdStorage() uint64
	SetMoneyRequiredFromColdStorage(amount uint64) error
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
