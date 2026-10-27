CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS trips (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_number   TEXT NOT NULL,
    source        TEXT NOT NULL,
    destination   TEXT NOT NULL,
    driver_name   TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'CREATED',
    version       INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT trips_trip_number_unique UNIQUE (trip_number),
    CONSTRAINT trips_status_check CHECK (
        status IN ('CREATED', 'ASSIGNED', 'IN_TRANSIT', 'COMPLETED', 'CANCELLED')
    ),
    CONSTRAINT trips_version_positive CHECK (version >= 1)
);

CREATE INDEX IF NOT EXISTS idx_trips_status_created_at
    ON trips (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_trips_created_at
    ON trips (created_at DESC);
