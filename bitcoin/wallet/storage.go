package wallet

import (
	"github.com/satori/go.uuid"
	"log"
)

type WalletStorage interface {
	GetLastSeenBlockHash() string
	SetLastSeenBlockHash(blockHash string)
	StoreTransaction(transaction *Transaction) *Transaction
	GetTransaction(hash string) *Transaction
	GetTransactionsWithLessConfirmations(confirmations int64) []*Transaction
	updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64)

	GetAccountByAddress(address string) *Account
	StoreAccount(account *Account)
}

type InMemoryWalletStorage struct {
	lastSeenBlockHash string
	accounts          []*Account
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

func (s *InMemoryWalletStorage) StoreTransaction(transaction *Transaction) *Transaction {
	existingTransaction := s.GetTransaction(transaction.Hash)
	if existingTransaction != nil {
		existingTransaction.update(transaction)
		return existingTransaction
	}
	log.Printf("New tx %s", transaction.Hash)
	transaction.Id = uuid.Must(uuid.NewV4())
	s.transactions = append(s.transactions, transaction)
	return transaction
}

func (s *InMemoryWalletStorage) GetAccountByAddress(address string) *Account {
	for _, account := range s.accounts {
		if account.Address == address {
			return account
		}
	}
	return nil
}

func (s *InMemoryWalletStorage) StoreAccount(account *Account) {
	s.accounts = append(s.accounts, account)
}

func (s *InMemoryWalletStorage) GetTransactionsWithLessConfirmations(confirmations int64) []*Transaction {
	result := make([]*Transaction, 0)

	for _, transaction := range s.transactions {
		if transaction.reportedConfirmations < confirmations {
			result = append(result, transaction)
		}
	}
	return result
}

func (s *InMemoryWalletStorage) updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) {
	storedTransaction := s.GetTransaction(transaction.Hash)
	storedTransaction.reportedConfirmations = reportedConfirmations
}

func newStorage(storageType string) WalletStorage {
	if storageType == "memory" {
		return &InMemoryWalletStorage{
			accounts:     make([]*Account, 0),
			transactions: make([]*Transaction, 0),
		}
	} else {
		log.Fatal("Error: unsupported storage type", storageType)
		return nil
	}
}
