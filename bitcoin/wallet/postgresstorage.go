package wallet

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/satori/go.uuid"

	"github.com/onederx/bitcoin-processing/settings"
)

type PostgresWalletStorage struct {
	db                *sql.DB
	lastSeenBlockHash string
}

type queryResult interface {
	Scan(dest ...interface{}) error
}

func newPostgresWalletStorage() *PostgresWalletStorage {
	lastSeenBlockHash := ""
	dsn := settings.GetStringMandatory("storage.dsn")

	db, err := sql.Open("postgres", dsn)

	if err != nil {
		log.Fatal(err)
	}

	err = db.QueryRow(`SELECT value FROM metadata
		WHERE key = 'last_seen_block_hash'`).Scan(&lastSeenBlockHash)

	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	storage := &PostgresWalletStorage{
		db:                db,
		lastSeenBlockHash: lastSeenBlockHash,
	}
	return storage
}

func (s *PostgresWalletStorage) GetLastSeenBlockHash() string {
	return s.lastSeenBlockHash
}

func (s *PostgresWalletStorage) SetLastSeenBlockHash(hash string) error {
	_, err := s.db.Exec(`INSERT INTO metadata (key, value)
		VALUES ('last_seen_block_hash', $1)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		hash,
	)
	if err != nil {
		return err
	}
	s.lastSeenBlockHash = hash
	return nil
}

func transactionFromDatabaseRow(row queryResult) (*Transaction, error) {
	var id uuid.UUID
	var hash, blockHash, address, direction string
	var confirmations, reportedConfirmations int64
	var amount uint64

	err := row.Scan(
		&id,
		&hash,
		&blockHash,
		&confirmations,
		&address,
		&direction,
		&amount,
		&reportedConfirmations,
	)

	if err != nil {
		return nil, err
	}
	tx := &Transaction{
		Id:                    id,
		Hash:                  hash,
		BlockHash:             blockHash,
		Confirmations:         confirmations,
		Address:               address,
		Direction:             TransactionDirectionFromString(direction),
		Amount:                amount,
		reportedConfirmations: reportedConfirmations,
	}
	return tx, nil
}

func (s *PostgresWalletStorage) GetTransaction(hash string) (*Transaction, error) {
	row := s.db.QueryRow(`SELECT id, hash, block_hash, confirmations, address,
		direction, amount, reported_confirmations
		FROM transactions WHERE hash = $1`,
		hash,
	)
	return transactionFromDatabaseRow(row)
}

func (s *PostgresWalletStorage) GetTransactionById(id uuid.UUID) (*Transaction, error) {
	row := s.db.QueryRow(`SELECT id, hash, block_hash, confirmations, address,
		direction, amount, reported_confirmations
		FROM transactions WHERE id = $1`,
		id,
	)
	return transactionFromDatabaseRow(row)
}

func (s *PostgresWalletStorage) StoreTransaction(transaction *Transaction) (*Transaction, error) {
	existingTransaction, err := s.GetTransaction(transaction.Hash)
	switch err {
	case nil: // tx already in database
		_, err := s.db.Exec(`UPDATE transactions SET block_hash = $1,
			confirmations = $2 WHERE id = $3`,
			transaction.BlockHash,
			transaction.Confirmations,
			existingTransaction.Id,
		)
		if err != nil {
			return nil, err
		}
		existingTransaction.update(transaction)
		return existingTransaction, nil
	case sql.ErrNoRows: // new tx
		log.Printf("New tx %s", transaction.Hash)
		transaction.Id = uuid.Must(uuid.NewV4())
		_, err := s.db.Exec(`INSERT INTO transactions (id, hash, block_hash,
			confirmations, address, direction, amount, reported_confirmations)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			transaction.Id,
			transaction.Hash,
			transaction.BlockHash,
			transaction.Confirmations,
			transaction.Address,
			transaction.Direction.String(),
			transaction.Amount,
			transaction.reportedConfirmations,
		)
		if err != nil {
			return nil, err
		}
		return transaction, nil
	default:
		return nil, err
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
	_, err = s.db.Exec(`INSERT INTO accounts (address, metainfo)
		VALUES ($1, $2)`, account.Address, marshaledMetainfo,
	)
	if err != nil {
		return err
	}
	return nil

}

func (s *PostgresWalletStorage) GetTransactionsWithLessConfirmations(confirmations int64) ([]*Transaction, error) {
	result := make([]*Transaction, 0, 20)

	rows, err := s.db.Query(`SELECT id, hash, block_hash, confirmations,
		address, direction, amount, reported_confirmations FROM transactions
        WHERE confirmations < $1`, confirmations,
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

func (s *PostgresWalletStorage) updateReportedConfirmations(transaction *Transaction, reportedConfirmations int64) error {
	_, err := s.db.Exec(`UPDATE transactions SET reported_confirmations = $1
		WHERE id = $2`, reportedConfirmations, transaction.Id)
	if err != nil {
		return err
	}
	transaction.reportedConfirmations = reportedConfirmations
	return nil
}
