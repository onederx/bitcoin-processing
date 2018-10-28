package wallet

import (
	"github.com/onederx/bitcoin-processing/settings"
	"github.com/satori/go.uuid"
	"log"
)

type WalletStorage interface {
	GetLastSeenBlockHash() string
	SetLastSeenBlockHash(string)
	StoreTransaction(*Transaction)
	GetTransaction(hash string) *Transaction
}

var storage WalletStorage

type InMemoryWalletStorage struct {
	lastSeenBlockHash string
	accounts          []Account
	transactions      []*Transaction
}

func (s *InMemoryWalletStorage) GetLastSeenBlockHash() string {
	return s.lastSeenBlockHash
}

func (s *InMemoryWalletStorage) SetLastSeenBlockHash(hash string) {
	s.lastSeenBlockHash = hash
}

func (s *InMemoryWalletStorage) GetTransaction(hash string) *Transaction {
	for _, transaction := range s.transactions {
		if transaction.Hash == hash {
			return transaction
		}
	}
	return nil
}

func (s *InMemoryWalletStorage) StoreTransaction(transaction *Transaction) {
	existingTransaction := s.GetTransaction(transaction.Hash)
	if existingTransaction != nil {
		existingTransaction.update(transaction)
	}
	transaction.id = uuid.Must(uuid.NewV4())
	s.transactions = append(s.transactions, transaction)
}

func initStorage() {
	storageType := settings.GetStringMandatory("storage.type")

	if storageType == "memory" {
		storage = &InMemoryWalletStorage{
			accounts:     make([]Account, 0),
			transactions: make([]*Transaction, 0),
		}
	} else {
		log.Fatal("Error: unsupported storage type", storageType)
	}
}
