\c bitcoin_processing;

CREATE TABLE IF NOT EXISTS bitcoin_processing_events (
    seq SERIAL PRIMARY KEY, # XXX: Warning: this can create gaps in case of rollback
    type TEXT NOT NULL,
    data JSONB
);