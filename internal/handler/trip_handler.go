package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/iamadarsh0309/Trip-API/internal/dto"
	"github.com/iamadarsh0309/Trip-API/internal/httperr"
	"github.com/iamadarsh0309/Trip-API/internal/service"
)

type TripHandler struct {
	svc *service.TripService
}

func NewTripHandler(svc *service.TripService) *TripHandler {
	return &TripHandler{svc: svc}
}

// CreateTrip godoc
// @Summary Create a trip
// @Description Creates a new trip. Requires Idempotency-Key header to prevent duplicate creates.
// @Tags trips
// @Accept json
// @Produce json
// @Param Idempotency-Key header string true "Client-generated idempotency key (16-128 chars)"
// @Param body body dto.CreateTripRequest true "Trip payload"
// @Success 201 {object} dto.TripResponse
// @Success 200 {object} dto.TripResponse "Idempotent replay of a previous successful create"
// @Failure 400 {object} httperr.Envelope
// @Failure 409 {object} httperr.Envelope
// @Failure 500 {object} httperr.Envelope
// @Router /trips [post]
func (h *TripHandler) CreateTrip(c *gin.Context) {
	var req dto.CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c, err)
		return
	}

	result, err := h.svc.Create(c.Request.Context(), c.GetHeader("Idempotency-Key"), req)
	if err != nil {
		writeError(c, err)
		return
	}

	c.Data(result.StatusCode, "application/json", result.Body)
}

// ListTrips godoc
// @Summary List trips
// @Description Returns paginated trips with optional status filter
// @Tags trips
// @Produce json
// @Param status query string false "Filter by status" Enums(CREATED,ASSIGNED,IN_TRANSIT,COMPLETED,CANCELLED)
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} dto.ListTripsResponse
// @Failure 400 {object} httperr.Envelope
// @Failure 500 {object} httperr.Envelope
// @Router /trips [get]
func (h *TripHandler) ListTrips(c *gin.Context) {
	var query dto.ListTripsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		writeValidationError(c, err)
		return
	}

	resp, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetTrip godoc
// @Summary Get trip by ID
// @Tags trips
// @Produce json
// @Param id path string true "Trip UUID"
// @Success 200 {object} dto.TripResponse
// @Failure 400 {object} httperr.Envelope
// @Failure 404 {object} httperr.Envelope
// @Failure 500 {object} httperr.Envelope
// @Router /trips/{id} [get]
func (h *TripHandler) GetTrip(c *gin.Context) {
	trip, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromDomain(trip))
}

// UpdateTripStatus godoc
// @Summary Update trip status
// @Description Enforces valid status transitions under row-level locking
// @Tags trips
// @Accept json
// @Produce json
// @Param id path string true "Trip UUID"
// @Param body body dto.UpdateStatusRequest true "New status"
// @Success 200 {object} dto.TripResponse
// @Failure 400 {object} httperr.Envelope
// @Failure 404 {object} httperr.Envelope
// @Failure 409 {object} httperr.Envelope
// @Failure 500 {object} httperr.Envelope
// @Router /trips/{id}/status [patch]
func (h *TripHandler) UpdateTripStatus(c *gin.Context) {
	var req dto.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeValidationError(c, err)
		return
	}

	trip, err := h.svc.UpdateStatus(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromDomain(trip))
}

// DeleteTrip godoc
// @Summary Delete a trip
// @Tags trips
// @Param id path string true "Trip UUID"
// @Success 204 "No Content"
// @Failure 400 {object} httperr.Envelope
// @Failure 404 {object} httperr.Envelope
// @Failure 500 {object} httperr.Envelope
// @Router /trips/{id} [delete]
func (h *TripHandler) DeleteTrip(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Health godoc
// @Summary Health check
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} httperr.Envelope
// @Router /health [get]
func (h *TripHandler) Health(c *gin.Context) {
	if err := h.svc.Health(c.Request.Context()); err != nil {
		writeError(c, httperr.New(http.StatusServiceUnavailable, httperr.CodeInternalError, "database unavailable"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func writeValidationError(c *gin.Context, err error) {
	details := []string{}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			details = append(details, fe.Field()+" failed on '"+fe.Tag()+"' validation")
		}
	} else if err.Error() != "" {
		details = append(details, err.Error())
	}
	writeError(c, httperr.Validation("invalid request", details...))
}
