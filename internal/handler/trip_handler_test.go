package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamadarsh0309/Trip-API/internal/domain"
	"github.com/iamadarsh0309/Trip-API/internal/repository"
	"github.com/iamadarsh0309/Trip-API/internal/service"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type handlerMockRepo struct {
	trips map[uuid.UUID]*domain.Trip
	keys  map[string]repository.IdempotencyRecord
}

func newHandlerMockRepo() *handlerMockRepo {
	return &handlerMockRepo{
		trips: make(map[uuid.UUID]*domain.Trip),
		keys:  make(map[string]repository.IdempotencyRecord),
	}
}

func (m *handlerMockRepo) Ping(ctx context.Context) error { return nil }

func (m *handlerMockRepo) CreateWithIdempotency(
	ctx context.Context,
	trip *domain.Trip,
	idempotencyKey string,
	requestHash string,
	responseBuilder func(*domain.Trip) (int, []byte, error),
) (*domain.Trip, error) {
	if rec, ok := m.keys[idempotencyKey]; ok {
		return nil, &repository.IdempotencyReplayError{Record: rec}
	}
	for _, existing := range m.trips {
		if existing.TripNumber == trip.TripNumber {
			return nil, repository.ErrTripNumberConflict
		}
	}

	created := &domain.Trip{
		ID:          uuid.New(),
		TripNumber:  trip.TripNumber,
		Source:      trip.Source,
		Destination: trip.Destination,
		DriverName:  trip.DriverName,
		Status:      domain.StatusCreated,
		Version:     1,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	code, body, err := responseBuilder(created)
	if err != nil {
		return nil, err
	}
	m.trips[created.ID] = created
	m.keys[idempotencyKey] = repository.IdempotencyRecord{
		Key:          idempotencyKey,
		RequestHash:  requestHash,
		TripID:       &created.ID,
		ResponseCode: code,
		ResponseBody: body,
		CreatedAt:    time.Now().UTC(),
	}
	return created, nil
}

func (m *handlerMockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	t, ok := m.trips[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (m *handlerMockRepo) List(ctx context.Context, status *domain.Status, limit, offset int) ([]domain.Trip, int64, error) {
	items := make([]domain.Trip, 0)
	for _, t := range m.trips {
		if status != nil && t.Status != *status {
			continue
		}
		items = append(items, *t)
	}
	total := int64(len(items))
	if offset >= len(items) {
		return []domain.Trip{}, total, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], total, nil
}

func (m *handlerMockRepo) UpdateStatus(ctx context.Context, id uuid.UUID, next domain.Status) (*domain.Trip, domain.Status, error) {
	t, ok := m.trips[id]
	if !ok {
		return nil, "", repository.ErrNotFound
	}
	current := t.Status
	if current == next {
		cp := *t
		return &cp, current, nil
	}
	if err := domain.ValidateTransition(current, next); err != nil {
		return nil, current, err
	}
	t.Status = next
	t.Version++
	t.UpdatedAt = time.Now().UTC()
	cp := *t
	return &cp, current, nil
}

func (m *handlerMockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.trips[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.trips, id)
	return nil
}

func setupRouter() (*gin.Engine, *handlerMockRepo) {
	repo := newHandlerMockRepo()
	svc := service.NewTripService(repo)
	h := NewTripHandler(svc)
	return NewRouter(h), repo
}

func TestHTTP_CreateGetListUpdateDeleteFlow(t *testing.T) {
	r, _ := setupRouter()

	body := `{"trip_number":"TRIP-2001","source":"Pune","destination":"Mumbai","driver_name":"Asha"}`
	req := httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idempotency-key-http1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, _ := created["id"].(string)
	if id == "" || created["status"] != "CREATED" {
		t.Fatalf("unexpected create payload: %v", created)
	}

	// idempotent replay
	req = httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idempotency-key-http1")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("replay want 200 got %d", w.Code)
	}

	// invalid transition
	req = httptest.NewRequest(http.MethodPatch, "/trips/"+id+"/status", bytes.NewBufferString(`{"status":"COMPLETED"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("invalid transition want 409 got %d body=%s", w.Code, w.Body.String())
	}

	// valid transition
	req = httptest.NewRequest(http.MethodPatch, "/trips/"+id+"/status", bytes.NewBufferString(`{"status":"ASSIGNED"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("assign want 200 got %d body=%s", w.Code, w.Body.String())
	}

	// list filter
	req = httptest.NewRequest(http.MethodGet, "/trips?status=ASSIGNED&page=1&page_size=10", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200 got %d", w.Code)
	}
	var list map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if int(list["total"].(float64)) != 1 {
		t.Fatalf("expected total 1, got %v", list)
	}

	// get
	req = httptest.NewRequest(http.MethodGet, "/trips/"+id, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get want 200 got %d", w.Code)
	}

	// delete
	req = httptest.NewRequest(http.MethodDelete, "/trips/"+id, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete want 204 got %d", w.Code)
	}

	// get after delete
	req = httptest.NewRequest(http.MethodGet, "/trips/"+id, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete want 404 got %d", w.Code)
	}
}

func TestHTTP_CreateValidationAndMissingIdempotencyKey(t *testing.T) {
	r, _ := setupRouter()

	req := httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idempotency-key-http2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for empty body, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(
		`{"trip_number":"T1","source":"A","destination":"B","driver_name":"C"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 missing idempotency key, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHTTP_TripNumberConflict(t *testing.T) {
	r, _ := setupRouter()
	payload := `{"trip_number":"TRIP-DUP","source":"A","destination":"B","driver_name":"C"}`

	req := httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idempotency-key-dup-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create want 201 got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idempotency-key-dup-2")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate trip_number want 409 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHTTP_Health(t *testing.T) {
	r, _ := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health want 200 got %d", w.Code)
	}
}

func TestHTTP_CancelTransition(t *testing.T) {
	r, _ := setupRouter()
	body := `{"trip_number":"TRIP-CANCEL","source":"A","destination":"B","driver_name":"C"}`
	req := httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idempotency-key-cancel1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"].(string)

	req = httptest.NewRequest(http.MethodPatch, "/trips/"+id+"/status", bytes.NewBufferString(`{"status":"CANCELLED"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel want 200 got %d body=%s", w.Code, w.Body.String())
	}

	// terminal: cannot move again
	req = httptest.NewRequest(http.MethodPatch, "/trips/"+id+"/status", bytes.NewBufferString(`{"status":"ASSIGNED"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("terminal transition want 409 got %d", w.Code)
	}
}
