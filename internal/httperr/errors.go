package httperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	CodeValidationError         Code = "VALIDATION_ERROR"
	CodeNotFound                Code = "NOT_FOUND"
	CodeInvalidStatusTransition Code = "INVALID_STATUS_TRANSITION"
	CodeTripNumberConflict      Code = "TRIP_NUMBER_CONFLICT"
	CodeIdempotencyKeyReused    Code = "IDEMPOTENCY_KEY_REUSED"
	CodeMissingIdempotencyKey   Code = "MISSING_IDEMPOTENCY_KEY"
	CodeInternalError           Code = "INTERNAL_ERROR"
	CodeBadRequest              Code = "BAD_REQUEST"
)

type AppError struct {
	HTTPStatus int
	Code       Code
	Message    string
	Details    []string
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(status int, code Code, message string, details ...string) *AppError {
	if details == nil {
		details = []string{}
	}
	return &AppError{
		HTTPStatus: status,
		Code:       code,
		Message:    message,
		Details:    details,
	}
}

func Validation(message string, details ...string) *AppError {
	return New(http.StatusBadRequest, CodeValidationError, message, details...)
}

func BadRequest(code Code, message string, details ...string) *AppError {
	return New(http.StatusBadRequest, code, message, details...)
}

func NotFound(message string) *AppError {
	return New(http.StatusNotFound, CodeNotFound, message)
}

func Conflict(code Code, message string, details ...string) *AppError {
	return New(http.StatusConflict, code, message, details...)
}

func Internal(message string) *AppError {
	return New(http.StatusInternalServerError, CodeInternalError, message)
}

func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

type Envelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    Code     `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
}

func ToEnvelope(err *AppError) Envelope {
	details := err.Details
	if details == nil {
		details = []string{}
	}
	return Envelope{
		Error: ErrorBody{
			Code:    err.Code,
			Message: err.Message,
			Details: details,
		},
	}
}
