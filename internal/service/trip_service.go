package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/iamadarsh0309/Trip-API/internal/domain"
	"github.com/iamadarsh0309/Trip-API/internal/dto"
	"github.com/iamadarsh0309/Trip-API/internal/httperr"
	"github.com/iamadarsh0309/Trip-API/internal/repository"
)

type TripRepository interface {
	Ping(ctx context.Context) error
	CreateWithIdempotency(
		ctx context.Context,
		trip *domain.Trip,
		idempotencyKey string,
		requestHash string,
		responseBuilder func(*domain.Trip) (int, []byte, error),
	) (*domain.Trip, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error)
	List(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CreateResult struct {
	Trip       *domain.Trip
	StatusCode int
	Body       []byte
	Replayed   bool
}

type TripService struct {
	repo TripRepository
}

func NewTripService(repo TripRepository) *TripService {
	return &TripService{repo: repo}
}

func (s *TripService) Health(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

func (s *TripService) Create(ctx context.Context, idempotencyKey string, req dto.CreateTripRequest) (*CreateResult, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return nil, httperr.BadRequest(httperr.CodeMissingIdempotencyKey, "Idempotency-Key header is required")
	}
	if n := utf8.RuneCountInString(key); n < 16 || n > 128 {
		return nil, httperr.Validation("Idempotency-Key must be between 16 and 128 characters")
	}

	req.TripNumber = strings.TrimSpace(req.TripNumber)
	req.Source = strings.TrimSpace(req.Source)
	req.Destination = strings.TrimSpace(req.Destination)
	req.DriverName = strings.TrimSpace(req.DriverName)

	hash, err := hashCreateRequest(req)
	if err != nil {
		return nil, httperr.Internal("failed to hash request")
	}

	trip := &domain.Trip{
		TripNumber:  req.TripNumber,
		Source:      req.Source,
		Destination: req.Destination,
		DriverName:  req.DriverName,
		Status:      domain.StatusCreated,
	}

	created, err := s.repo.CreateWithIdempotency(ctx, trip, key, hash, func(t *domain.Trip) (int, []byte, error) {
		body, err := json.Marshal(dto.FromDomain(t))
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, body, nil
	})
	if err != nil {
		var replay *repository.IdempotencyReplayError
		if errors.As(err, &replay) {
			if replay.Record.RequestHash != hash {
				return nil, httperr.Conflict(
					httperr.CodeIdempotencyKeyReused,
					"Idempotency-Key was already used with a different request body",
				)
			}
			// Idempotent replay returns 200 with the original response body.
			return &CreateResult{
				StatusCode: http.StatusOK,
				Body:       replay.Record.ResponseBody,
				Replayed:   true,
			}, nil
		}
		if errors.Is(err, repository.ErrTripNumberConflict) {
			return nil, httperr.Conflict(httperr.CodeTripNumberConflict, "trip_number already exists")
		}
		if errors.Is(err, repository.ErrIdempotencyConflict) {
			return nil, httperr.Conflict(httperr.CodeIdempotencyKeyReused, "concurrent idempotency key conflict; retry the request")
		}
		return nil, httperr.Internal("failed to create trip")
	}

	body, err := json.Marshal(dto.FromDomain(created))
	if err != nil {
		return nil, httperr.Internal("failed to encode response")
	}

	return &CreateResult{
		Trip:       created,
		StatusCode: http.StatusCreated,
		Body:       body,
		Replayed:   false,
	}, nil
}

func (s *TripService) GetByID(ctx context.Context, idStr string) (*domain.Trip, error) {
	id, err := parseUUID(idStr)
	if err != nil {
		return nil, err
	}
	trip, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, httperr.NotFound("trip not found")
	}
	if err != nil {
		return nil, httperr.Internal("failed to get trip")
	}
	return trip, nil
}

func (s *TripService) List(ctx context.Context, query dto.ListTripsQuery) (*dto.ListTripsResponse, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, httperr.Validation("page_size must be between 1 and 100")
	}

	var statusFilter *domain.Status
	if strings.TrimSpace(query.Status) != "" {
		st, err := domain.ParseStatus(strings.TrimSpace(query.Status))
		if err != nil {
			return nil, httperr.Validation("invalid status filter", query.Status)
		}
		statusFilter = &st
	}

	offset := (page - 1) * pageSize
	trips, total, err := s.repo.List(ctx, statusFilter, pageSize, offset)
	if err != nil {
		return nil, httperr.Internal("failed to list trips")
	}

	return &dto.ListTripsResponse{
		Items:    dto.FromDomainList(trips),
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *TripService) UpdateStatus(ctx context.Context, idStr string, req dto.UpdateStatusRequest) (*domain.Trip, error) {
	id, err := parseUUID(idStr)
	if err != nil {
		return nil, err
	}

	next, err := domain.ParseStatus(strings.TrimSpace(req.Status))
	if err != nil {
		return nil, httperr.Validation("invalid status", req.Status)
	}

	trip, current, err := s.repo.UpdateStatus(ctx, id, next)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, httperr.NotFound("trip not found")
	}
	var transitionErr *domain.InvalidTransitionError
	if errors.As(err, &transitionErr) {
		return nil, httperr.Conflict(
			httperr.CodeInvalidStatusTransition,
			transitionErr.Error(),
			fmt.Sprintf("from=%s", current),
			fmt.Sprintf("to=%s", next),
		)
	}
	if err != nil {
		return nil, httperr.Internal("failed to update trip status")
	}
	return trip, nil
}

func (s *TripService) Delete(ctx context.Context, idStr string) error {
	id, err := parseUUID(idStr)
	if err != nil {
		return err
	}
	err = s.repo.Delete(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return httperr.NotFound("trip not found")
	}
	if err != nil {
		return httperr.Internal("failed to delete trip")
	}
	return nil
}

func parseUUID(idStr string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(idStr))
	if err != nil {
		return uuid.Nil, httperr.Validation("id must be a valid UUID")
	}
	return id, nil
}

func hashCreateRequest(req dto.CreateTripRequest) (string, error) {
	canonical := struct {
		TripNumber  string `json:"trip_number"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
		DriverName  string `json:"driver_name"`
	}{
		TripNumber:  req.TripNumber,
		Source:      req.Source,
		Destination: req.Destination,
		DriverName:  req.DriverName,
	}
	b, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
