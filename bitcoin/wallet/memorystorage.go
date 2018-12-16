package wallet

import (
	"errors"
	"log"

	"github.com/satori/go.uuid"
)

// InMemoryWalletStorage is a Storage implementation that stores data in memory.
// It does not provide any king of persistence, safety or efficiency (all
// methods are implemented naively) and exists only for testing purposes (to get
// a working Storage implementation without external dependencies). Only
// PostgresWalletStorage should be used in production.
//
// In future InMemoryWalletStorage may be used as a cache in front of
// PostgresWalletStorage to make storage faster, but it is not implemented now
type InMemoryWalletStorage struct {
	lastSeenBlockHash            string
	accounts                     []*Account
	transactions                 []*Transaction
	hotWalletAddress             string
	moneyRequiredFromColdStorage uint64
}

// GetLastSeenBlockHash returns last seen block hash - a string set by
// SetLastSeenBlockHash
func (s *InMemoryWalletStorage) GetLastSeenBlockHash() string {
	return s.lastSeenBlockHash
}

// SetLastSeenBlockHash sets last seen block hash - a string returned by
// GetLastSeenBlockHash
func (s *InMemoryWalletStorage) SetLastSeenBlockHash(hash string) error {
	s.lastSeenBlockHash = hash
	return nil
}

// GetMoneyRequiredFromColdStorage returns money required to transfer from
// cold storage - uint64 value set by SetMoneyRequiredFromColdStorage
func (s *InMemoryWalletStorage) GetMoneyRequiredFromColdStorage() uint64 {
	return s.moneyRequiredFromColdStorage
}

// SetMoneyRequiredFromColdStorage stores money required to transfer from
// cold storage - uint64 value returned by GetMoneyRequiredFromColdStorage
func (s *InMemoryWalletStorage) SetMoneyRequiredFromColdStorage(amount uint64) error {
	s.moneyRequiredFromColdStorage = amount
	return nil
}

// GetTransactionByHash fetches first transaction which bitcoin tx hash equals
// given value. In theory there can be multiple txns with same hash (referring
// to same bitcoin tx) - currently, this will happen in case of internal
// transfer, when one exchange client transfers money to another. From the
// wallet's point of view, it is a transfer from one in-wallet address to
// another and will create both incoming and outgoing tx.
func (s *InMemoryWalletStorage) GetTransactionByHash(hash string) (*Transaction, error) {
	for _, transaction := range s.transactions {
		if transaction.Hash == hash {
			return transaction, nil
		}
	}
	return nil, errors.New("Transaction with hash " + hash + " not found")
}

// GetTransactionByID fetches tx given it's internal id (uuid assigned by
// exchange or processing app)
func (s *InMemoryWalletStorage) GetTransactionByID(id uuid.UUID) (*Transaction, error) {
	for _, transaction := range s.transactions {
		if transaction.ID == id {
			return transaction, nil
		}
	}
	return nil, errors.New("Transaction with id " + id.String() + " not found")
}

// StoreTransaction stores new tx or updates existing one in storage.
// For the update case txns are matched using bitcoin tx hash
// This method is most probably outdated and may work incorrectly for internal
// transfers or other txns
func (s *InMemoryWalletStorage) StoreTransaction(transaction *Transaction) (*Transaction, error) {
	existingTransaction, err := s.GetTransactionByHash(transaction.Hash)
	if err != nil {
		return nil, err
	}

	if existingTransaction != nil {
		transaction.fresh = false
		existingTransaction.update(transaction)
		return existingTransaction, nil
	}

	transaction.fresh = true
	if transaction.ID == uuid.Nil {
		if transaction.Direction == OutgoingDirection {
			log.Printf(
				"Warning: generating new id for new unseen outgoing tx. "+
					"This should not happen because outgoing transactions are"+
					" generated by us. Tx: %v",
				transaction,
			)
		}
		transaction.ID = uuid.Must(uuid.NewV4())
	}
	s.transactions = append(s.transactions, transaction)
	return transaction, nil
}

// GetAccountByAddress fetches Account info that has given address
func (s *InMemoryWalletStorage) GetAccountByAddress(address string) (*Account, error) {
	for _, account := range s.accounts {
		if account.Address == address {
			return account, nil
		}
	}
	return nil, errors.New("Account with address " + address + " not found")
}

// StoreAccount stores Account information. No checks, including checks for
// address duplication, are performed
func (s *InMemoryWalletStorage) StoreAccount(account *Account) error {
	s.accounts = append(s.accounts, account)
	return nil
}

// GetBroadcastedTransactionsWithLessConfirmations returns txns which are
// already broadcasted to Bitcoin network (have corresponding Bitcoin tx), but
// still have less than given number of confirmations. This method is used by
// wallet updater to get txns for which updated info should be requested from
// Bitcoin node. When tx reaches max confirmations (this value is set in
// config as 'transaction.max_confirmations', 6 by default), it is considered
// fully confirmed and updater won't request further updates on it
func (s *InMemoryWalletStorage) GetBroadcastedTransactionsWithLessConfirmations(confirmations int64) ([]*Transaction, error) {
	result := make([]*Transaction, 0)

	for _, transaction := range s.transactions {
		if transaction.Hash == "" {
			continue
		}
		if transaction.reportedConfirmations < confirmations {
			result = append(result, transaction)
		}
	}

	return result, nil
}

func (s *InMemoryWalletStorage) updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) error {
	storedTransaction, err := s.GetTransactionByHash(transaction.Hash)
	if err != nil {
		return err
	}

	storedTransaction.reportedConfirmations = reportedConfirmations
	return nil
}

// GetHotWalletAddress returns hot wallet address - string value set by
// SetHotWalletAddress
func (s *InMemoryWalletStorage) GetHotWalletAddress() string {
	return s.hotWalletAddress
}

// SetHotWalletAddress sets hot wallet address - string value returned by
// GetHotWalletAddress
func (s *InMemoryWalletStorage) SetHotWalletAddress(address string) error {
	s.hotWalletAddress = address
	return nil
}

// GetPendingTransactions returns txns referring to withdrawals with status
// 'pending' or 'pending-cold-storage' - in other words, withdrawals for which
// there is not enough confirmed balance to fund right now. This function is
// used by wallet updater to update their statuses and compute money required
// from cold storage. Txns with status 'pending-manual-confirmation' are NOT
// returned by this call.
func (s *InMemoryWalletStorage) GetPendingTransactions() ([]*Transaction, error) {
	result := make([]*Transaction, 0)

	for _, transaction := range s.transactions {
		status := transaction.Status
		if status == PendingTransaction || status == PendingColdStorageTransaction {
			result = append(result, transaction)
		}
	}

	return result, nil
}

// GetTransactionsWithFilter gets txns filtered by direction and/or status.
// Empty values of filters mean do not use this filter, with non-empty filter
// only txns that have equal value of corresponding parameter will be included
// in resulting slice
func (s *InMemoryWalletStorage) GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*Transaction, error) {
	result := make([]*Transaction, 0)

	for _, transaction := range s.transactions {
		if directionFilter != "" && directionFilter != transaction.Direction.String() {
			continue
		}

		if statusFilter != "" && statusFilter != transaction.Status.String() {
			continue
		}

		result = append(result, transaction)
	}

	return result, nil
}
