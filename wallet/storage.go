package wallet

import (
	"database/sql"
	"errors"
	"log"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/wallet/types"
)

var ErrHotWalletAddressNotGeneratedYet = errors.New("Hot wallet address not generated yet")

// Storage is responsible for storing and fetching wallet-related information:
// transactions, accounts, and various metainformation about current wallet or
// its state. Currently, metainformation includes hot wallet address, last seen
// bitcoin block hash and amount of money required to transfer from cold storage
type Storage interface {
	GetLastSeenBlockHash() (string, error)
	SetLastSeenBlockHash(blockHash string) error
	StoreTransaction(transaction *types.Transaction) (*types.Transaction, error)
	GetTransactionByHash(hash string) (*types.Transaction, error)
	GetTransactionByID(id uuid.UUID) (*types.Transaction, error)
	GetBroadcastedTransactionsWithLessConfirmations(confirmations int64) ([]*types.Transaction, error)
	GetPendingTransactions() ([]*types.Transaction, error)
	updateReportedConfirmations(transaction *types.Transaction, reportedConfirmations int64) error
	GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*types.Transaction, error)

	GetAccountByAddress(address string) (*Account, error)
	StoreAccount(account *Account) error

	GetHotWalletAddress() (string, error)
	setHotWalletAddress(address string) error

	GetMoneyRequiredFromColdStorage() (uint64, error)
	SetMoneyRequiredFromColdStorage(amount uint64) error

	LockWallet(operation interface{}) error
	ClearWallet() error
	CheckWalletLock() (bool, string, error)

	WithTransaction(sqlTX *sql.Tx) Storage
	CurrentTransaction() *sql.Tx
	GetDB() *sql.DB
}

func NewStorage(db *sql.DB) Storage {
	if db == nil {
		log.Print("Warning: initializing in-memory wallet storage since no db " +
			"connection is passed. Note it should not be used in production")
		return &InMemoryWalletStorage{
			accounts:     make([]*Account, 0),
			transactions: make([]*types.Transaction, 0),
		}
	}

	return newPostgresWalletStorage(db)
}
