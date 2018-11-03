\c bitcoin_processing;

CREATE TABLE IF NOT EXISTS events (
    seq SERIAL PRIMARY KEY, -- XXX: Warning: SERIAL can have gaps in case of rollback
    type TEXT NOT NULL,
    data JSONB
);