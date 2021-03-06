\c bitcoin_processing;

CREATE TABLE IF NOT EXISTS events (
    seq SERIAL PRIMARY KEY, -- XXX: Warning: SERIAL can have gaps in case of rollback
    type TEXT NOT NULL,
    data JSONB
);

CREATE TABLE IF NOT EXISTS accounts (
    address TEXT PRIMARY KEY,
    metainfo JSONB
);

CREATE TABLE IF NOT EXISTS transactions (
    id uuid PRIMARY KEY,
    hash TEXT,
    block_hash TEXT,
    confirmations BIGINT,
    address TEXT,
    direction TEXT,
    status TEXT,
    amount BIGINT, -- satoshis
    metainfo JSONB,
    fee BIGINT,
    fee_type TEXT,
    cold_storage BOOLEAN,
    reported_confirmations BIGINT
);

CREATE TABLE IF NOT EXISTS metadata (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS muted_events (
    tx_id uuid PRIMARY KEY
);