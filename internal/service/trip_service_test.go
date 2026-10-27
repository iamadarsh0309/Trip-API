package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamadarsh0309/Trip-API/internal/domain"
	"github.com/iamadarsh0309/Trip-API/internal/dto"
	"github.com/iamadarsh0309/Trip-API/internal/httperr"
	"github.com/iamadarsh0309/Trip-API/internal/repository"
)

type mockRepo struct {
	createFn       func(ctx context.Context, trip *domain.Trip, key, hash string, builder func(*domain.Trip) (int, []byte, error)) (*domain.Trip, error)
	getByIDFn      func(ctx context.Context, id uuid.UUID) (*domain.Trip, error)
	listFn         func(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error)
	updateStatusFn func(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error)
	deleteFn       func(ctx context.Context, id uuid.UUID) error
	pingErr        error
}

func (m *mockRepo) Ping(ctx context.Context) error { return m.pingErr }

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
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, repository.ErrNotFound
}

func (m *mockRepo) List(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error) {
	if m.listFn != nil {
		return m.listFn(ctx, status, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockRepo) UpdateStatus(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error) {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, next)
	}
	return nil, "", repository.ErrNotFound
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return repository.ErrNotFound
}

func sampleTrip() *domain.Trip {
	now := time.Now().UTC()
	return &domain.Trip{
		ID:          uuid.New(),
		TripNumber:  "TRIP-1001",
		Source:      "Bangalore",
		Destination: "Hyderabad",
		DriverName:  "Rahul",
		Status:      domain.StatusCreated,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
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

func TestCreate_RejectsShortIdempotencyKey(t *testing.T) {
	svc := NewTripService(&mockRepo{})
	_, err := svc.Create(context.Background(), "too-short", dto.CreateTripRequest{
		TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D",
	})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeValidationError {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreate_Success(t *testing.T) {
	created := sampleTrip()
	svc := NewTripService(&mockRepo{
		createFn: func(ctx context.Context, trip *domain.Trip, key, hash string, builder func(*domain.Trip) (int, []byte, error)) (*domain.Trip, error) {
			if key != "idempotency-key-001" {
				t.Fatalf("unexpected key %s", key)
			}
			if trip.TripNumber != "TRIP-1001" {
				t.Fatalf("unexpected trip number %s", trip.TripNumber)
			}
			code, body, err := builder(created)
			if err != nil || code != http.StatusCreated || len(body) == 0 {
				t.Fatalf("builder failed: %d %v %v", code, body, err)
			}
			return created, nil
		},
	})

	res, err := svc.Create(context.Background(), "idempotency-key-001", dto.CreateTripRequest{
		TripNumber: "  TRIP-1001  ", Source: " Bangalore ", Destination: "Hyderabad", DriverName: "Rahul",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Replayed || res.StatusCode != http.StatusCreated || res.Trip.ID != created.ID {
		t.Fatalf("unexpected result: %+v", res)
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

func TestGetByID(t *testing.T) {
	trip := sampleTrip()
	svc := NewTripService(&mockRepo{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
			if id != trip.ID {
				t.Fatalf("unexpected id %s", id)
			}
			return trip, nil
		},
	})

	got, err := svc.GetByID(context.Background(), trip.ID.String())
	if err != nil || got.ID != trip.ID {
		t.Fatalf("unexpected get: %v %v", got, err)
	}

	_, err = svc.GetByID(context.Background(), "not-a-uuid")
	if appErr, ok := httperr.AsAppError(err); !ok || appErr.Code != httperr.CodeValidationError {
		t.Fatalf("expected validation for bad uuid, got %v", err)
	}

	_, err = NewTripService(&mockRepo{}).GetByID(context.Background(), uuid.NewString())
	if appErr, ok := httperr.AsAppError(err); !ok || appErr.Code != httperr.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestList_DefaultsAndFilter(t *testing.T) {
	trip := sampleTrip()
	trip.Status = domain.StatusAssigned

	svc := NewTripService(&mockRepo{
		listFn: func(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error) {
			if status == nil || *status != domain.StatusAssigned {
				t.Fatalf("expected ASSIGNED filter, got %v", status)
			}
			if limit != 20 || offset != 0 {
				t.Fatalf("unexpected pagination limit=%d offset=%d", limit, offset)
			}
			return []domain.Trip{*trip}, 1, nil
		},
	})

	resp, err := svc.List(context.Background(), dto.ListTripsQuery{Status: "ASSIGNED"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Page != 1 || resp.PageSize != 20 || resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected list response: %+v", resp)
	}
}

func TestList_PageSizeTooLarge(t *testing.T) {
	svc := NewTripService(&mockRepo{})
	_, err := svc.List(context.Background(), dto.ListTripsQuery{PageSize: 101})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeValidationError {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestList_InvalidStatusFilter(t *testing.T) {
	svc := NewTripService(&mockRepo{})
	_, err := svc.List(context.Background(), dto.ListTripsQuery{Status: "FLYING"})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeValidationError {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestList_OffsetCalculation(t *testing.T) {
	svc := NewTripService(&mockRepo{
		listFn: func(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error) {
			if limit != 10 || offset != 20 {
				t.Fatalf("want limit=10 offset=20, got %d %d", limit, offset)
			}
			return []domain.Trip{}, 0, nil
		},
	})
	_, err := svc.List(context.Background(), dto.ListTripsQuery{Page: 3, PageSize: 10})
	if err != nil {
		t.Fatal(err)
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

func TestUpdateStatus_SuccessAndInvalidStatus(t *testing.T) {
	trip := sampleTrip()
	trip.Status = domain.StatusAssigned
	svc := NewTripService(&mockRepo{
		updateStatusFn: func(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error) {
			return trip, domain.StatusCreated, nil
		},
	})

	got, err := svc.UpdateStatus(context.Background(), trip.ID.String(), dto.UpdateStatusRequest{Status: "ASSIGNED"})
	if err != nil || got.Status != domain.StatusAssigned {
		t.Fatalf("unexpected update: %v %v", got, err)
	}

	_, err = svc.UpdateStatus(context.Background(), trip.ID.String(), dto.UpdateStatusRequest{Status: "NOPE"})
	if appErr, ok := httperr.AsAppError(err); !ok || appErr.Code != httperr.CodeValidationError {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	id := uuid.New()
	deleted := false
	svc := NewTripService(&mockRepo{
		deleteFn: func(ctx context.Context, got uuid.UUID) error {
			if got != id {
				t.Fatalf("unexpected id")
			}
			deleted = true
			return nil
		},
	})
	if err := svc.Delete(context.Background(), id.String()); err != nil || !deleted {
		t.Fatalf("delete failed: %v", err)
	}

	err := NewTripService(&mockRepo{}).Delete(context.Background(), uuid.NewString())
	if appErr, ok := httperr.AsAppError(err); !ok || appErr.Code != httperr.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestHealth(t *testing.T) {
	if err := NewTripService(&mockRepo{}).Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := NewTripService(&mockRepo{pingErr: context.DeadlineExceeded}).Health(context.Background())
	if err == nil {
		t.Fatal("expected ping error")
	}
}

func TestHashCreateRequest_StableAndSensitive(t *testing.T) {
	h1, err := hashCreateRequest(dto.CreateTripRequest{TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D"})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := hashCreateRequest(dto.CreateTripRequest{TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D"})
	if err != nil {
		t.Fatal(err)
	}
	h3, err := hashCreateRequest(dto.CreateTripRequest{TripNumber: "T1", Source: "A", Destination: "B", DriverName: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || h1 == h3 || len(h1) != 64 {
		t.Fatalf("hash behavior unexpected: %s %s %s", h1, h2, h3)
	}
}

func TestCreate_RejectsTooLongIdempotencyKey(t *testing.T) {
	svc := NewTripService(&mockRepo{})
	longKey := strings.Repeat("k", 129)
	_, err := svc.Create(context.Background(), longKey, dto.CreateTripRequest{
		TripNumber: "T1", Source: "A", Destination: "B", DriverName: "D",
	})
	appErr, ok := httperr.AsAppError(err)
	if !ok || appErr.Code != httperr.CodeValidationError {
		t.Fatalf("expected validation error, got %v", err)
	}
}
