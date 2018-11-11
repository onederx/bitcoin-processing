package wallet

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/settings"
)

type PostgresWalletStorage struct {
	db                *sql.DB
	lastSeenBlockHash string
	hotWalletAddress  string
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
	reported_confirmations
`

func newPostgresWalletStorage() *PostgresWalletStorage {
	dsn := settings.GetStringMandatory("storage.dsn")

	db, err := sql.Open("postgres", dsn)

	if err != nil {
		log.Fatal(err)
	}

	storage := &PostgresWalletStorage{
		db: db,
	}

	storage.lastSeenBlockHash, err = storage.getMeta("last_seen_block_hash", "")

	if err != nil {
		log.Fatal(err)
	}

	storage.hotWalletAddress, err = storage.getMeta("hot_wallet_address", "")

	if err != nil {
		log.Fatal(err)
	}

	return storage
}

func (s *PostgresWalletStorage) getMeta(name string, defaultVal string) (string, error) {
	result := defaultVal
	err := s.db.QueryRow(`SELECT value FROM metadata
		WHERE key = $1`, name).Scan(&result)
	if err == nil || err == sql.ErrNoRows {
		return result, nil
	}
	return "", err
}

func (s *PostgresWalletStorage) setMeta(name string, value string) error {
	_, err := s.db.Exec(`INSERT INTO metadata (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		name,
		value,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *PostgresWalletStorage) GetLastSeenBlockHash() string {
	return s.lastSeenBlockHash
}

func (s *PostgresWalletStorage) SetLastSeenBlockHash(hash string) error {
	err := s.setMeta("last_seen_block_hash", hash)
	if err != nil {
		return err
	}
	s.lastSeenBlockHash = hash
	return nil
}

func transactionFromDatabaseRow(row queryResult) (*Transaction, error) {
	var id uuid.UUID
	var hash, blockHash, address, direction, status, feeType string
	var metainfoJSON *string
	var confirmations, reportedConfirmations int64
	var amount, fee uint64
	var metainfo interface{}

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
		&reportedConfirmations,
	)

	if err != nil {
		return nil, err
	}
	transactionDirection, err := TransactionDirectionFromString(direction)
	if err != nil {
		return nil, err
	}
	transactionStatus, err := TransactionStatusFromString(status)
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
	tx := &Transaction{
		Id:                    id,
		Hash:                  hash,
		BlockHash:             blockHash,
		Confirmations:         confirmations,
		Address:               address,
		Direction:             transactionDirection,
		Status:                transactionStatus,
		Amount:                amount,
		Metainfo:              metainfo,
		Fee:                   fee,
		FeeType:               transactionFeeType,
		fresh:                 false,
		reportedConfirmations: reportedConfirmations,
	}
	return tx, nil
}

func (s *PostgresWalletStorage) GetTransactionByHash(hash string) (*Transaction, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE hash = $1`,
		transactionFields,
	)
	row := s.db.QueryRow(query, hash)
	return transactionFromDatabaseRow(row)
}

func (s *PostgresWalletStorage) GetTransactionById(id uuid.UUID) (*Transaction, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE id = $1`,
		transactionFields,
	)
	row := s.db.QueryRow(query, id)

	return transactionFromDatabaseRow(row)
}

func (s *PostgresWalletStorage) StoreTransaction(transaction *Transaction) (*Transaction, error) {
	var existingTransaction *Transaction
	var err error

	txIsNew := true

	if transaction.Id != uuid.Nil {
		existingTransaction, err = s.GetTransactionById(transaction.Id)
		switch err {
		case nil: // tx already in database
			txIsNew = false
		case sql.ErrNoRows: // new tx
		default:
			return nil, err
		}
	}

	if txIsNew && transaction.Hash != "" {
		existingTransaction, err = s.GetTransactionByHash(transaction.Hash)
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
			existingTransaction.Id,
		)
		if err != nil {
			return nil, errors.New(fmt.Sprintf(
				"Update of tx data in DB failed: %s. Tx %#v",
				err,
				transaction,
			))
		}
		existingTransaction.update(transaction)
		return existingTransaction, nil
	} else {
		if transaction.Id == uuid.Nil {
			if transaction.Direction == OutgoingDirection {
				log.Printf(
					"Warning: generating new id for new unseen outgoing tx. "+
						"This should not happen because outgoing transactions are"+
						" generated by us. Tx: %v",
					transaction,
				)
				debug.PrintStack()
			}
			transaction.Id = uuid.Must(uuid.NewV4())
		}
		metainfoJSON, err := json.Marshal(transaction.Metainfo)
		if err != nil {
			return nil, err
		}
		query := fmt.Sprintf(`INSERT INTO transactions (%s)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			transactionFields,
		)
		_, err = s.db.Exec(
			query,
			transaction.Id,
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
			transaction.reportedConfirmations,
		)
		if err != nil {
			return nil, errors.New(fmt.Sprintf(
				"Failed to insert new tx into DB: %s. Tx %#v",
				err,
				transaction,
			))
		}
		return transaction, nil
	}
}

func (s *PostgresWalletStorage) GetAccountByAddress(address string) (*Account, error) {
	var marshaledMetainfo string
	var metainfo map[string]interface{}
	err := s.db.QueryRow(
		"SELECT metainfo FROM accounts WHERE address = $1",
		address,
	).Scan(&marshaledMetainfo)
	if err != nil {
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
	if err != nil {
		return err
	}
	return nil

}

func (s *PostgresWalletStorage) GetBroadcastedTransactionsWithLessConfirmations(confirmations int64) ([]*Transaction, error) {
	result := make([]*Transaction, 0, 20)

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
	err = rows.Err()
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *PostgresWalletStorage) updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) error {
	_, err := s.db.Exec(
		`UPDATE transactions SET reported_confirmations = $1 WHERE id = $2`,
		reportedConfirmations,
		transaction.Id,
	)
	if err != nil {
		return err
	}
	transaction.reportedConfirmations = reportedConfirmations
	return nil
}

func (s *PostgresWalletStorage) GetHotWalletAddress() string {
	return s.hotWalletAddress
}

func (s *PostgresWalletStorage) SetHotWalletAddress(address string) error {
	err := s.setMeta("hot_wallet_address", address)
	if err != nil {
		return err
	}
	s.hotWalletAddress = address
	return nil
}

func (s *PostgresWalletStorage) GetPendingTransactions() ([]*Transaction, error) {
	result := make([]*Transaction, 0, 20)

	query := fmt.Sprintf(
		`SELECT %s FROM transactions WHERE status = $1 OR status = $2`,
		transactionFields,
	)
	rows, err := s.db.Query(
		query,
		PendingTransaction.String(),
		PendingColdStorageTransaction.String(),
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
	err = rows.Err()
	if err != nil {
		return result, err
	}
	return result, nil

}
