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
    confirmations bigint,
    address TEXT,
    direction TEXT,
    reported_confirmations bigint
);

CREATE TABLE IF NOT EXISTS metadata (
    key TEXT PRIMARY KEY,
    value TEXT
)