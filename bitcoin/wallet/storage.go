package wallet

import (
	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/settings"
)

// Storage is responsible for storing and fetching wallet-related information:
// transactions, accounts, and various metainformation about current wallet or
// its state. Currently, metainformation includes hot wallet address, last seen
// bitcoin block hash and amount of money required to transfer from cold storage
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

func NewStorage(storageType string, s settings.Settings) Storage {
	var storage Storage

	switch storageType {
	case "memory":
		storage = &InMemoryWalletStorage{
			accounts:     make([]*Account, 0),
			transactions: make([]*Transaction, 0),
		}
	case "postgres":
		storage = newPostgresWalletStorage(s)
	default:
		panic("Error: unsupported storage type " + storageType)
	}

	return storage
}
