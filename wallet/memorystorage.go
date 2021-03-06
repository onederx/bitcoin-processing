package wallet

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"runtime/debug"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/wallet/types"
)

type ErrNoTxWithSuchID uuid.UUID

func (e ErrNoTxWithSuchID) Error() string {
	return "Transaction with id " + uuid.UUID(e).String() + " not found"
}

var errNoTxWithSuchTxHashAndDirection = errors.New(
	"Transaction with such hash, direction and address is not in db",
)

// InMemoryWalletStorage is a Storage implementation that stores data in memory.
// It does not provide any kind of persistence, safety or efficiency (all
// methods are implemented naively) and exists only for testing purposes (to get
// a working Storage implementation without external dependencies). Only
// PostgresWalletStorage should be used in production.
//
// In future InMemoryWalletStorage may be used as a cache in front of
// PostgresWalletStorage to make storage faster, but it is not implemented now
type InMemoryWalletStorage struct {
	lastSeenBlockHash            string
	accounts                     []*Account
	transactions                 []*types.Transaction
	hotWalletAddress             string
	moneyRequiredFromColdStorage uint64
	walletOperationLock          string
	hotWalletAddressWasSet       bool
}

// GetLastSeenBlockHash returns last seen block hash - a string set by
// SetLastSeenBlockHash
func (s *InMemoryWalletStorage) GetLastSeenBlockHash() (string, error) {
	return s.lastSeenBlockHash, nil
}

// SetLastSeenBlockHash sets last seen block hash - a string returned by
// GetLastSeenBlockHash
func (s *InMemoryWalletStorage) SetLastSeenBlockHash(hash string) error {
	s.lastSeenBlockHash = hash
	return nil
}

// GetMoneyRequiredFromColdStorage returns money required to transfer from
// cold storage - uint64 value set by SetMoneyRequiredFromColdStorage
func (s *InMemoryWalletStorage) GetMoneyRequiredFromColdStorage() (uint64, error) {
	return s.moneyRequiredFromColdStorage, nil
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
func (s *InMemoryWalletStorage) GetTransactionByHash(hash string) (*types.Transaction, error) {
	for _, transaction := range s.transactions {
		if transaction.Hash == hash {
			return transaction, nil
		}
	}
	return nil, errors.New("Transaction with hash " + hash + " not found")
}

// GetTransactionByID fetches tx given it's internal id (uuid assigned by
// exchange or processing app)
func (s *InMemoryWalletStorage) GetTransactionByID(id uuid.UUID) (*types.Transaction, error) {
	for _, transaction := range s.transactions {
		if transaction.ID == id {
			return transaction, nil
		}
	}
	return nil, ErrNoTxWithSuchID(id)
}

// GetTransactionByHashDirectionAndAddress fetches tx which same bitcoin tx
// hash, direction and address as given one
func (s *InMemoryWalletStorage) GetTransactionByHashDirectionAndAddress(tx *types.Transaction) (*types.Transaction, error) {
	for _, transaction := range s.transactions {
		if transaction.Hash == tx.Hash && transaction.Direction == tx.Direction && transaction.Address == tx.Address {
			return transaction, nil
		}
	}
	return nil, errNoTxWithSuchTxHashAndDirection
}

// StoreTransaction stores new tx or updates existing one in storage.
// For the update case txns are matched using bitcoin tx hash
// This method is most probably outdated and may work incorrectly for internal
// transfers or other txns
func (s *InMemoryWalletStorage) StoreTransaction(transaction *types.Transaction) (*types.Transaction, error) {
	var existingTransaction *types.Transaction
	var err error

	txIsNew := true

	if transaction.ID != uuid.Nil {
		existingTransaction, err = s.GetTransactionByID(transaction.ID)

		if err == nil {
			// tx already in database
			txIsNew = false
		} else {
			if _, ok := err.(ErrNoTxWithSuchID); !ok {
				return nil, err
			}
		}
	}

	if txIsNew && transaction.Hash != "" {
		existingTransaction, err = s.GetTransactionByHashDirectionAndAddress(transaction)
		switch err {
		case nil: // tx already in database
			txIsNew = false
		case errNoTxWithSuchTxHashAndDirection: // new tx
		default:
			return nil, err
		}
	}

	if !txIsNew {
		existingTransaction.Update(transaction)
		return existingTransaction, nil
	}

	if transaction.ID == uuid.Nil {
		if transaction.Direction == types.OutgoingDirection {
			log.Printf(
				"Warning: generating new id for new unseen outgoing tx. "+
					"This should not happen because outgoing transactions are"+
					" generated by us. Tx: %v",
				transaction,
			)
			debug.PrintStack()
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
	return nil, nil
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
func (s *InMemoryWalletStorage) GetBroadcastedTransactionsWithLessConfirmations(confirmations int64) ([]*types.Transaction, error) {
	result := make([]*types.Transaction, 0)

	for _, transaction := range s.transactions {
		if transaction.Hash == "" {
			continue
		}
		if transaction.ReportedConfirmations < confirmations {
			result = append(result, transaction)
		}
	}

	return result, nil
}

func (s *InMemoryWalletStorage) updateReportedConfirmations(transaction *types.Transaction, reportedConfirmations int64) error {
	storedTransaction, err := s.GetTransactionByID(transaction.ID)
	if err != nil {
		return err
	}

	storedTransaction.ReportedConfirmations = reportedConfirmations
	return nil
}

// GetHotWalletAddress returns hot wallet address - string value set by
// SetHotWalletAddress
func (s *InMemoryWalletStorage) GetHotWalletAddress() (string, error) {
	if s.hotWalletAddress == "" && !s.hotWalletAddressWasSet {
		return "", ErrHotWalletAddressNotGeneratedYet
	}
	return s.hotWalletAddress, nil
}

// SetHotWalletAddress sets hot wallet address - string value returned by
// GetHotWalletAddress
func (s *InMemoryWalletStorage) setHotWalletAddress(address string) error {
	s.hotWalletAddress = address
	s.hotWalletAddressWasSet = true
	return nil
}

// GetPendingTransactions returns txns referring to withdrawals with status
// 'pending' or 'pending-cold-storage' - in other words, withdrawals for which
// there is not enough confirmed balance to fund right now. This function is
// used by wallet updater to update their statuses and compute money required
// from cold storage. Txns with status 'pending-manual-confirmation' are NOT
// returned by this call.
func (s *InMemoryWalletStorage) GetPendingTransactions() ([]*types.Transaction, error) {
	result := make([]*types.Transaction, 0)

	for _, transaction := range s.transactions {
		status := transaction.Status
		if status == types.PendingTransaction || status == types.PendingColdStorageTransaction {
			result = append(result, transaction)
		}
	}

	return result, nil
}

// GetTransactionsWithFilter gets txns filtered by direction and/or status.
// Empty values of filters mean do not use this filter, with non-empty filter
// only txns that have equal value of corresponding parameter will be included
// in resulting slice
func (s *InMemoryWalletStorage) GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*types.Transaction, error) {
	result := make([]*types.Transaction, 0)

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

func (s *InMemoryWalletStorage) WithTransaction(sqlTX *sql.Tx) Storage {
	log.Printf(
		"Warning: WithTransaction called on memory wallet storage. Memory " +
			"storage does not support transactions, so it just does nothing.",
	)
	return s
}

func (s *InMemoryWalletStorage) CurrentTransaction() *sql.Tx {
	log.Printf(
		"Warning: CurrentTransaction called on memory wallet storage. Memory " +
			"storage does not support transactions, so it always returns nil.",
	)
	return nil
}

func (s *InMemoryWalletStorage) GetDB() *sql.DB {
	return nil
}
func (s *InMemoryWalletStorage) LockWallet(operation interface{}) error {
	operationMarshaled, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	s.walletOperationLock = string(operationMarshaled)
	return nil
}

func (s *InMemoryWalletStorage) ClearWallet() error {
	s.walletOperationLock = ""
	return nil
}

func (s *InMemoryWalletStorage) CheckWalletLock() (bool, string, error) {
	operation := s.walletOperationLock
	return operation == "", operation, nil
}
