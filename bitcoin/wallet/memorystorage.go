package wallet

import (
	"errors"
	"github.com/satori/go.uuid"
)

type InMemoryWalletStorage struct {
	lastSeenBlockHash string
	accounts          []*Account
	transactions      []*Transaction
	hotWalletAddress  string
}

func (s *InMemoryWalletStorage) GetLastSeenBlockHash() string {
	return s.lastSeenBlockHash
}

func (s *InMemoryWalletStorage) SetLastSeenBlockHash(hash string) error {
	s.lastSeenBlockHash = hash
	return nil
}

func (s *InMemoryWalletStorage) GetTransaction(hash string) (*Transaction, error) {
	for _, transaction := range s.transactions {
		if transaction.Hash == hash {
			return transaction, nil
		}
	}
	return nil, errors.New("Transaction with hash " + hash + " not found")
}

func (s *InMemoryWalletStorage) StoreTransaction(transaction *Transaction) (*Transaction, error) {
	existingTransaction, err := s.GetTransaction(transaction.Hash)
	if err != nil {
		return nil, err
	}
	if existingTransaction != nil {
		transaction.fresh = false
		existingTransaction.update(transaction)
		return existingTransaction, nil
	}
	transaction.fresh = true
	if transaction.Id == uuid.Nil {
		transaction.Id = uuid.Must(uuid.NewV4())
	}
	s.transactions = append(s.transactions, transaction)
	return transaction, nil
}

func (s *InMemoryWalletStorage) GetAccountByAddress(address string) (*Account, error) {
	for _, account := range s.accounts {
		if account.Address == address {
			return account, nil
		}
	}
	return nil, errors.New("Account with address " + address + " not found")
}

func (s *InMemoryWalletStorage) StoreAccount(account *Account) error {
	s.accounts = append(s.accounts, account)
	return nil
}

func (s *InMemoryWalletStorage) GetTransactionsWithLessConfirmations(confirmations int64) ([]*Transaction, error) {
	result := make([]*Transaction, 0)

	for _, transaction := range s.transactions {
		if transaction.reportedConfirmations < confirmations {
			result = append(result, transaction)
		}
	}
	return result, nil
}

func (s *InMemoryWalletStorage) updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) error {
	storedTransaction, err := s.GetTransaction(transaction.Hash)
	if err != nil {
		return err
	}
	storedTransaction.reportedConfirmations = reportedConfirmations
	return nil
}

func (s *InMemoryWalletStorage) GetHotWalletAddress() string {
	return s.hotWalletAddress
}

func (s *InMemoryWalletStorage) SetHotWalletAddress(address string) error {
	s.hotWalletAddress = address
	return nil
}
