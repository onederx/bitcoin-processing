package wallet

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/storage"
	"github.com/onederx/bitcoin-processing/wallet/types"
)

// PostgresWalletStorage is a Storage implementation that stores data in
// postgresql database. It is a primary Storage implementation that should be
// used in production. Currently, most methods are implemented by directly
// making SQL queries to DB and returning their results.
type PostgresWalletStorage struct {
	db storage.SQLQueryExecutor
}

type queryResult interface {
	Scan(dest ...interface{}) error
}

const transactionFields string = `
	id,
	hash,
	block_hash,
	confirmations,
	address,
	direction,
	status,
	amount,
	metainfo,
	fee,
	fee_type,
	cold_storage,
	reported_confirmations
`

func newPostgresWalletStorage(db *sql.DB) *PostgresWalletStorage {
	storage := &PostgresWalletStorage{db: db}
	return storage
}

func (s *PostgresWalletStorage) getMeta(name string, defaultVal string) (string, error) {
	return storage.GetMeta(s.db, name, defaultVal)
}

func (s *PostgresWalletStorage) setMeta(name string, value string) error {
	return storage.SetMeta(s.db, name, value)
}

// GetLastSeenBlockHash returns last seen block hash - a string set by
// SetLastSeenBlockHash. "Last seen block hash" is a hash of the last bitcoin
// block processed, processing needs to store it to distinguish seen blocks
// and request only new info from bitcoin node. The return value can be an
// empty string in case to seen block hash was stored (by SetLastSeenBlockHash)
// yet. This is considered normal and can happen in case processing was started
// for the first time and has not seen any blocks yet.
func (s *PostgresWalletStorage) GetLastSeenBlockHash() (string, error) {
	return s.getMeta("last_seen_block_hash", "")
}

// SetLastSeenBlockHash sets last seen block hash - a string returned by
// GetLastSeenBlockHash. The value is written to DB.
func (s *PostgresWalletStorage) SetLastSeenBlockHash(hash string) error {
	return s.setMeta("last_seen_block_hash", hash)
}

func transactionFromDatabaseRow(row queryResult) (*types.Transaction, error) {
	var id uuid.UUID
	var hash, blockHash, address, direction, status, feeType string
	var metainfoJSON *string
	var confirmations, reportedConfirmations int64
	var amount, fee uint64
	var metainfo interface{}
	var coldStorage bool

	err := row.Scan(
		&id,
		&hash,
		&blockHash,
		&confirmations,
		&address,
		&direction,
		&status,
		&amount,
		&metainfoJSON,
		&fee,
		&feeType,
		&coldStorage,
		&reportedConfirmations,
	)
	if err != nil {
		return nil, err
	}

	transactionDirection, err := types.TransactionDirectionFromString(direction)
	if err != nil {
		return nil, err
	}
	transactionStatus, err := types.TransactionStatusFromString(status)
	if err != nil {
		return nil, err
	}
	transactionFeeType, _ := bitcoin.FeeTypeFromString(feeType)
	if metainfoJSON != nil {
		err = json.Unmarshal([]byte(*metainfoJSON), &metainfo)
		if err != nil {
			return nil, err
		}
	} else {
		metainfo = nil
	}

	tx := &types.Transaction{
		ID:                    id,
		Hash:                  hash,
		BlockHash:             blockHash,
		Confirmations:         confirmations,
		Address:               address,
		Direction:             transactionDirection,
		Status:                transactionStatus,
		Amount:                bitcoin.BTCAmount(amount),
		Metainfo:              metainfo,
		Fee:                   bitcoin.BTCAmount(fee),
		FeeType:               transactionFeeType,
		ColdStorage:           coldStorage,
		Fresh:                 false,
		ReportedConfirmations: reportedConfirmations,
	}
	return tx, nil
}

// GetTransactionByHash fetches first transaction which bitcoin tx hash equals
// given value. In theory there can be multiple txns with same hash (referring
// to same bitcoin tx) - currently, this will happen in case of internal
// transfer, when one exchange client transfers money to another. From the
// wallet's point of view, it is a transfer from one in-wallet address to
// another and will create both incoming and outgoing tx.
func (s *PostgresWalletStorage) GetTransactionByHash(hash string) (*types.Transaction, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE hash = $1`,
		transactionFields,
	)
	row := s.db.QueryRow(query, hash)
	return transactionFromDatabaseRow(row)
}

// GetTransactionByHashDirectionAndAddress fetches tx which same bitcoin tx
// hash, direction and address as given one
func (s *PostgresWalletStorage) GetTransactionByHashDirectionAndAddress(tx *types.Transaction) (*types.Transaction, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE hash = $1 and direction = $2 and
		address = $3`,
		transactionFields,
	)
	row := s.db.QueryRow(
		query,
		tx.Hash,
		tx.Direction.String(),
		tx.Address,
	)
	return transactionFromDatabaseRow(row)
}

// GetTransactionByID fetches tx given it's internal id (uuid assigned by
// exchange or processing app), which is a private key in transactions table
func (s *PostgresWalletStorage) GetTransactionByID(id uuid.UUID) (*types.Transaction, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE id = $1`,
		transactionFields,
	)
	row := s.db.QueryRow(query, id)
	return transactionFromDatabaseRow(row)
}

// StoreTransaction stores new tx or updates existing one in storage. Firstly,
// a check is performed whether such tx already exists: if it has uuid assigned,
// it is searched by it. This can happen when some part of processing app
// updates info about a tx - for example, when processing app is able to
// process pending tx it updates it changing status and adding Bitcoin tx hash.
// Additionally, it is checked if there is a tx with equal Bitcoin tx hash and
// direction. This can match in case update on this tx arrived from Bitcoin
// node (it gained more confirmations).
// If tx was not found by either ways, new record is created. If tx had no ID,
// new uuid is generated. This only normal for incoming txns, interface for
// outgoing txns assumes ID is already provided by client (if it was not sent
// in /withdraw request, api package should have generated it itself)
func (s *PostgresWalletStorage) StoreTransaction(transaction *types.Transaction) (*types.Transaction, error) {
	var existingTransaction *types.Transaction
	var err error

	txIsNew := true

	if transaction.ID != uuid.Nil {
		existingTransaction, err = s.GetTransactionByID(transaction.ID)
		switch err {
		case nil: // tx already in database
			txIsNew = false
		case sql.ErrNoRows: // new tx
		default:
			return nil, err
		}
	}

	if txIsNew && transaction.Hash != "" {
		existingTransaction, err = s.GetTransactionByHashDirectionAndAddress(transaction)
		switch err {
		case nil: // tx already in database
			txIsNew = false
		case sql.ErrNoRows: // new tx
		default:
			return nil, err
		}
	}

	if !txIsNew {
		_, err := s.db.Exec(`UPDATE transactions SET hash = $1, block_hash = $2,
			confirmations = $3, status = $4 WHERE id = $5`,
			transaction.Hash,
			transaction.BlockHash,
			transaction.Confirmations,
			transaction.Status.String(),
			existingTransaction.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("Update of tx data in DB failed: %s. Tx %#v",
				err, transaction)
		}
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
	metainfoJSON, err := json.Marshal(transaction.Metainfo)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`INSERT INTO transactions (%s)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		transactionFields,
	)
	_, err = s.db.Exec(
		query,
		transaction.ID,
		transaction.Hash,
		transaction.BlockHash,
		transaction.Confirmations,
		transaction.Address,
		transaction.Direction.String(),
		transaction.Status.String(),
		transaction.Amount,
		string(metainfoJSON),
		transaction.Fee,
		transaction.FeeType.String(),
		transaction.ColdStorage,
		transaction.ReportedConfirmations,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to insert new tx into DB: %s. Tx %#v",
			err, transaction)
	}
	return transaction, nil
}

// GetAccountByAddress fetches account metainfo corresponding to given address
// and returns resulting Account structure
func (s *PostgresWalletStorage) GetAccountByAddress(address string) (*Account, error) {
	var marshaledMetainfo string
	var metainfo map[string]interface{}
	err := s.db.QueryRow(
		"SELECT metainfo FROM accounts WHERE address = $1",
		address,
	).Scan(&marshaledMetainfo)

	switch err {
	case nil:
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
	err = json.Unmarshal([]byte(marshaledMetainfo), &metainfo)
	if err != nil {
		return nil, err
	}
	account := &Account{
		Address:  address,
		Metainfo: metainfo,
	}
	return account, nil
}

// StoreAccount stores a new account record with account address (which is
// a private key) and metainfo
func (s *PostgresWalletStorage) StoreAccount(account *Account) error {
	marshaledMetainfo, err := json.Marshal(account.Metainfo)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO accounts (address, metainfo) VALUES ($1, $2)`,
		account.Address,
		marshaledMetainfo,
	)
	return err
}

// GetBroadcastedTransactionsWithLessConfirmations returns txns which are
// already broadcasted to Bitcoin network (have corresponding Bitcoin tx), but
// still have less than given number of confirmations. This method is used by
// wallet updater to get txns for which updated info should be requested from
// Bitcoin node. When tx reaches max confirmations (this value is set in
// config as 'transaction.max_confirmations', 6 by default), it is considered
// fully confirmed and updater won't request further updates on it
func (s *PostgresWalletStorage) GetBroadcastedTransactionsWithLessConfirmations(confirmations int64) ([]*types.Transaction, error) {
	result := make([]*types.Transaction, 0, 20)

	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE confirmations < $1 and hash != ''`,
		transactionFields,
	)
	rows, err := s.db.Query(query, confirmations)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := transactionFromDatabaseRow(rows)
		if err != nil {
			return result, err
		}
		result = append(result, transaction)
	}
	return result, rows.Err()
}

func (s *PostgresWalletStorage) updateReportedConfirmations(transaction *types.Transaction, reportedConfirmations int64) error {
	_, err := s.db.Exec(
		`UPDATE transactions SET reported_confirmations = $1 WHERE id = $2`,
		reportedConfirmations,
		transaction.ID,
	)
	if err != nil {
		return err
	}
	transaction.ReportedConfirmations = reportedConfirmations
	return nil
}

// GetHotWalletAddress returns hot wallet address - string value set by
// SetHotWalletAddress.
func (s *PostgresWalletStorage) GetHotWalletAddress() (string, error) {
	address, err := s.getMeta("hot_wallet_address", "")

	if err != nil {
		return "", err
	}

	if address == "" {
		return "", ErrHotWalletAddressNotGeneratedYet
	}
	return address, nil
}

// SetHotWalletAddress sets hot wallet address - string value returned by
// GetHotWalletAddress. This operation makes an update in DB.
func (s *PostgresWalletStorage) setHotWalletAddress(address string) error {
	return s.setMeta("hot_wallet_address", address)
}

// GetPendingTransactions returns txns referring to withdrawals with status
// 'pending' or 'pending-cold-storage' - in other words, withdrawals for which
// there is not enough confirmed balance to fund right now. This function is
// used by wallet updater to update their statuses and compute money required
// from cold storage. Txns with status 'pending-manual-confirmation' are NOT
// returned by this call.
func (s *PostgresWalletStorage) GetPendingTransactions() ([]*types.Transaction, error) {
	result := make([]*types.Transaction, 0, 20)

	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE status = $1 OR status = $2`,
		transactionFields,
	)
	rows, err := s.db.Query(
		query,
		types.PendingTransaction.String(),
		types.PendingColdStorageTransaction.String(),
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := transactionFromDatabaseRow(rows)
		if err != nil {
			return result, err
		}
		result = append(result, transaction)
	}
	return result, rows.Err()
}

// GetMoneyRequiredFromColdStorage returns money required to transfer from
// cold storage - uint64 value set by SetMoneyRequiredFromColdStorage.
func (s *PostgresWalletStorage) GetMoneyRequiredFromColdStorage() (uint64, error) {
	moneyRequiredFromColdStorageString, err := s.getMeta(
		"money_required_from_cold_storage",
		"0",
	)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(moneyRequiredFromColdStorageString, 10, 64)
}

// SetMoneyRequiredFromColdStorage stores money required to transfer from
// cold storage - uint64 value returned by GetMoneyRequiredFromColdStorage. The
// value is updated in DB and in memory
func (s *PostgresWalletStorage) SetMoneyRequiredFromColdStorage(amount uint64) error {
	return s.setMeta(
		"money_required_from_cold_storage",
		strconv.FormatUint(amount, 10),
	)
}

// GetTransactionsWithFilter gets txns filtered by direction and/or status.
// Empty values of filters mean do not use this filter, with non-empty filter
// only txns that have equal value of corresponding parameter will be included
// in resulting slice
func (s *PostgresWalletStorage) GetTransactionsWithFilter(directionFilter string, statusFilter string) ([]*types.Transaction, error) {
	query := fmt.Sprintf("SELECT %s FROM transactions", transactionFields)
	queryArgs := make([]interface{}, 0, 2)
	whereClause := make([]string, 0, 2)
	argc := 0
	result := make([]*types.Transaction, 0, 20)

	if directionFilter != "" {
		argc++
		whereClause = append(whereClause, fmt.Sprintf("direction = $%d", argc))
		queryArgs = append(queryArgs, directionFilter)
	}
	if statusFilter != "" {
		argc++
		whereClause = append(whereClause, fmt.Sprintf("status = $%d", argc))
		queryArgs = append(queryArgs, statusFilter)
	}
	if len(whereClause) > 0 {
		query += " WHERE " + strings.Join(whereClause, " AND ")
	}
	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		transaction, err := transactionFromDatabaseRow(rows)
		if err != nil {
			return result, err
		}
		result = append(result, transaction)
	}
	err = rows.Err()
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *PostgresWalletStorage) WithTransaction(sqlTX *sql.Tx) Storage {
	return &PostgresWalletStorage{db: sqlTX}
}

func (s *PostgresWalletStorage) CurrentTransaction() *sql.Tx {
	sqlTX, _ := s.db.(*sql.Tx)
	return sqlTX
}

func (s *PostgresWalletStorage) GetDB() *sql.DB {
	return s.db.(*sql.DB)
}

func (s *PostgresWalletStorage) LockWallet(operation interface{}) error {
	operationMarshaled, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	return s.setMeta("wallet_operation", string(operationMarshaled))
}

func (s *PostgresWalletStorage) ClearWallet() error {
	return s.setMeta("wallet_operation", "")
}

func (s *PostgresWalletStorage) CheckWalletLock() (bool, string, error) {
	operation, err := s.getMeta("wallet_operation", "")
	return operation == "", operation, err
}
