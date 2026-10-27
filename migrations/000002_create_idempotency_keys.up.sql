CREATE TABLE IF NOT EXISTS idempotency_keys (
    key            TEXT PRIMARY KEY,
    request_hash   TEXT NOT NULL,
    trip_id        UUID REFERENCES trips(id) ON DELETE SET NULL,
    response_code  INTEGER NOT NULL,
    response_body  JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT idempotency_keys_key_length CHECK (char_length(key) BETWEEN 16 AND 128)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_trip_id
    ON idempotency_keys (trip_id);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_created_at
    ON idempotency_keys (created_at);
