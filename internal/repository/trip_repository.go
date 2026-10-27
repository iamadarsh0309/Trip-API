package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamadarsh0309/Trip-API/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrTripNumberConflict  = errors.New("trip number conflict")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)

type IdempotencyRecord struct {
	Key          string
	RequestHash  string
	TripID       *uuid.UUID
	ResponseCode int
	ResponseBody []byte
	CreatedAt    time.Time
}

type IdempotencyReplayError struct {
	Record IdempotencyRecord
}

func (e *IdempotencyReplayError) Error() string {
	return "idempotency key already used"
}

type TripRepository struct {
	pool *pgxpool.Pool
}

func NewTripRepository(pool *pgxpool.Pool) *TripRepository {
	return &TripRepository{pool: pool}
}

func (r *TripRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// CreateWithIdempotency inserts a trip and stores the idempotency response in one transaction.
// responseBuilder receives the persisted trip and must return HTTP status + JSON body to cache.
func (r *TripRepository) CreateWithIdempotency(
	ctx context.Context,
	trip *domain.Trip,
	idempotencyKey string,
	requestHash string,
	responseBuilder func(*domain.Trip) (int, []byte, error),
) (*domain.Trip, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var existing IdempotencyRecord
	err = tx.QueryRow(ctx, `
		SELECT key, request_hash, trip_id, response_code, response_body, created_at
		FROM idempotency_keys
		WHERE key = $1
		FOR UPDATE
	`, idempotencyKey).Scan(
		&existing.Key,
		&existing.RequestHash,
		&existing.TripID,
		&existing.ResponseCode,
		&existing.ResponseBody,
		&existing.CreatedAt,
	)
	if err == nil {
		return nil, &IdempotencyReplayError{Record: existing}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lookup idempotency key: %w", err)
	}

	created := &domain.Trip{}
	var status string
	err = tx.QueryRow(ctx, `
		INSERT INTO trips (trip_number, source, destination, driver_name, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, trip_number, source, destination, driver_name, status, version, created_at, updated_at
	`,
		trip.TripNumber,
		trip.Source,
		trip.Destination,
		trip.DriverName,
		string(domain.StatusCreated),
	).Scan(
		&created.ID,
		&created.TripNumber,
		&created.Source,
		&created.Destination,
		&created.DriverName,
		&status,
		&created.Version,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, "trips_trip_number_unique") {
			return nil, ErrTripNumberConflict
		}
		return nil, fmt.Errorf("insert trip: %w", err)
	}
	created.Status = domain.Status(status)

	responseCode, responseBody, err := responseBuilder(created)
	if err != nil {
		return nil, fmt.Errorf("build response: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_keys (key, request_hash, trip_id, response_code, response_body)
		VALUES ($1, $2, $3, $4, $5)
	`, idempotencyKey, requestHash, created.ID, responseCode, responseBody)
	if err != nil {
		if isUniqueViolation(err, "idempotency_keys_pkey") {
			return nil, ErrIdempotencyConflict
		}
		return nil, fmt.Errorf("insert idempotency key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create: %w", err)
	}
	return created, nil
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	return r.scanTrip(ctx, r.pool, `
		SELECT id, trip_number, source, destination, driver_name, status, version, created_at, updated_at
		FROM trips
		WHERE id = $1
	`, id)
}

func (r *TripRepository) List(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error) {
	var total int64
	var err error

	if status != nil {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trips WHERE status = $1`, string(*status)).Scan(&total)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trips`).Scan(&total)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("count trips: %w", err)
	}

	var rows pgx.Rows
	if status != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT id, trip_number, source, destination, driver_name, status, version, created_at, updated_at
			FROM trips
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`, string(*status), limit, offset)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, trip_number, source, destination, driver_name, status, version, created_at, updated_at
			FROM trips
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list trips: %w", err)
	}
	defer rows.Close()

	trips := make([]domain.Trip, 0)
	for rows.Next() {
		t, err := scanTripRow(rows)
		if err != nil {
			return nil, 0, err
		}
		trips = append(trips, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate trips: %w", err)
	}
	return trips, total, nil
}

func (r *TripRepository) UpdateStatus(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	trip, err := r.scanTrip(ctx, tx, `
		SELECT id, trip_number, source, destination, driver_name, status, version, created_at, updated_at
		FROM trips
		WHERE id = $1
		FOR UPDATE
	`, id)
	if err != nil {
		return nil, "", err
	}

	current := trip.Status
	if current == next {
		if err := tx.Commit(ctx); err != nil {
			return nil, "", fmt.Errorf("commit noop: %w", err)
		}
		return trip, current, nil
	}

	if err := domain.ValidateTransition(current, next); err != nil {
		return nil, current, err
	}

	updated := &domain.Trip{}
	var status string
	err = tx.QueryRow(ctx, `
		UPDATE trips
		SET status = $1,
		    version = version + 1,
		    updated_at = NOW()
		WHERE id = $2
		RETURNING id, trip_number, source, destination, driver_name, status, version, created_at, updated_at
	`, string(next), id).Scan(
		&updated.ID,
		&updated.TripNumber,
		&updated.Source,
		&updated.Destination,
		&updated.DriverName,
		&status,
		&updated.Version,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		return nil, current, fmt.Errorf("update status: %w", err)
	}
	updated.Status = domain.Status(status)

	if err := tx.Commit(ctx); err != nil {
		return nil, current, fmt.Errorf("commit status update: %w", err)
	}
	return updated, current, nil
}

func (r *TripRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM trips WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete trip: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *TripRepository) scanTrip(ctx context.Context, q querier, sql string, args ...any) (*domain.Trip, error) {
	t := &domain.Trip{}
	var status string
	err := q.QueryRow(ctx, sql, args...).Scan(
		&t.ID,
		&t.TripNumber,
		&t.Source,
		&t.Destination,
		&t.DriverName,
		&status,
		&t.Version,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan trip: %w", err)
	}
	t.Status = domain.Status(status)
	return t, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTripRow(row rowScanner) (*domain.Trip, error) {
	t := &domain.Trip{}
	var status string
	if err := row.Scan(
		&t.ID,
		&t.TripNumber,
		&t.Source,
		&t.Destination,
		&t.DriverName,
		&status,
		&t.Version,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan trip row: %w", err)
	}
	t.Status = domain.Status(status)
	return t, nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if constraint == "" || pgErr.ConstraintName == constraint {
			return true
		}
	}
	return false
}
