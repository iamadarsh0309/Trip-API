package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/iamadarsh0309/Trip-API/internal/domain"
	"github.com/iamadarsh0309/Trip-API/internal/dto"
	"github.com/iamadarsh0309/Trip-API/internal/httperr"
	"github.com/iamadarsh0309/Trip-API/internal/repository"
)

type mockRepo struct {
	createFn       func(ctx context.Context, trip *domain.Trip, key, hash string, builder func(*domain.Trip) (int, []byte, error)) (*domain.Trip, error)
	updateStatusFn func(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error)
}

func (m *mockRepo) Ping(ctx context.Context) error { return nil }

func (m *mockRepo) CreateWithIdempotency(
	ctx context.Context,
	trip *domain.Trip,
	idempotencyKey string,
	requestHash string,
	responseBuilder func(*domain.Trip) (int, []byte, error),
) (*domain.Trip, error) {
	return m.createFn(ctx, trip, idempotencyKey, requestHash, responseBuilder)
}

func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	return nil, repository.ErrNotFound
}

func (m *mockRepo) List(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error) {
	return nil, 0, nil
}

func (m *mockRepo) UpdateStatus(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error) {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, next)
	}
	return nil, "", repository.ErrNotFound
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return repository.ErrNotFound
}

func TestCreate_RequiresIdempotencyKey(t *testing.T) {
	svc := NewTripService(&mockRepo{})
	_, err := svc.Create(context.Background(), "", dto.CreateTripRequest{
		TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D",
	})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeMissingIdempotencyKey {
		t.Fatalf("expected missing idempotency key error, got %v", err)
	}
}

func TestCreate_IdempotentReplaySameBody(t *testing.T) {
	body, _ := json.Marshal(dto.TripResponse{ID: uuid.NewString(), TripNumber: "T1", Status: "CREATED"})
	hash, _ := hashCreateRequest(dto.CreateTripRequest{
		TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D",
	})

	svc := NewTripService(&mockRepo{
		createFn: func(ctx context.Context, trip *domain.Trip, key, reqHash string, builder func(*domain.Trip) (int, []byte, error)) (*domain.Trip, error) {
			return nil, &repository.IdempotencyReplayError{
				Record: repository.IdempotencyRecord{
					RequestHash:  hash,
					ResponseCode: http.StatusCreated,
					ResponseBody: body,
				},
			}
		},
	})

	res, err := svc.Create(context.Background(), "idempotency-key-001", dto.CreateTripRequest{
		TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Replayed || res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 replay, got %+v", res)
	}
}

func TestCreate_IdempotencyKeyReusedDifferentBody(t *testing.T) {
	svc := NewTripService(&mockRepo{
		createFn: func(ctx context.Context, trip *domain.Trip, key, reqHash string, builder func(*domain.Trip) (int, []byte, error)) (*domain.Trip, error) {
			return nil, &repository.IdempotencyReplayError{
				Record: repository.IdempotencyRecord{
					RequestHash:  "different-hash",
					ResponseCode: http.StatusCreated,
					ResponseBody: []byte(`{}`),
				},
			}
		},
	})

	_, err := svc.Create(context.Background(), "idempotency-key-001", dto.CreateTripRequest{
		TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D",
	})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeIdempotencyKeyReused {
		t.Fatalf("expected IDEMPOTENCY_KEY_REUSED, got %v", err)
	}
}

func TestUpdateStatus_InvalidTransitionMapped(t *testing.T) {
	id := uuid.New()
	svc := NewTripService(&mockRepo{
		updateStatusFn: func(ctx context.Context, got uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error) {
			return nil, domain.StatusCreated, &domain.InvalidTransitionError{From: domain.StatusCreated, To: next}
		},
	})

	_, err := svc.UpdateStatus(context.Background(), id.String(), dto.UpdateStatusRequest{Status: "COMPLETED"})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeInvalidStatusTransition {
		t.Fatalf("expected INVALID_STATUS_TRANSITION, got %v", err)
	}
}

func TestHashCreateRequest_Stable(t *testing.T) {
	h1, err := hashCreateRequest(dto.CreateTripRequest{TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D"})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := hashCreateRequest(dto.CreateTripRequest{TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D"})
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("hash should be stable")
	}
	if h1 == "" {
		t.Fatal("hash should not be empty")
	}
}

func TestCreate_TripNumberConflict(t *testing.T) {
	svc := NewTripService(&mockRepo{
		createFn: func(ctx context.Context, trip *domain.Trip, key, hash string, builder func(*domain.Trip) (int, []byte, error)) (*domain.Trip, error) {
			return nil, repository.ErrTripNumberConflict
		},
	})
	_, err := svc.Create(context.Background(), "idempotency-key-001", dto.CreateTripRequest{
		TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D",
	})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeTripNumberConflict {
		t.Fatalf("expected TRIP_NUMBER_CONFLICT, got %v", err)
	}
}
