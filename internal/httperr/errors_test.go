package httperr

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppErrorHelpers(t *testing.T) {
	cases := []struct {
		name   string
		err    *AppError
		status int
		code   Code
	}{
		{"validation", Validation("bad", "field"), http.StatusBadRequest, CodeValidationError},
		{"not_found", NotFound("missing"), http.StatusNotFound, CodeNotFound},
		{"conflict", Conflict(CodeTripNumberConflict, "dup"), http.StatusConflict, CodeTripNumberConflict},
		{"internal", Internal("boom"), http.StatusInternalServerError, CodeInternalError},
		{"bad_request", BadRequest(CodeMissingIdempotencyKey, "need key"), http.StatusBadRequest, CodeMissingIdempotencyKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.HTTPStatus != tc.status || tc.err.Code != tc.code {
				t.Fatalf("unexpected error: %+v", tc.err)
			}
			if tc.err.Error() == "" {
				t.Fatal("expected non-empty Error()")
			}
		})
	}
}

func TestAsAppError(t *testing.T) {
	app := NotFound("x")
	got, ok := AsAppError(app)
	if !ok || got.Code != CodeNotFound {
		t.Fatalf("expected AppError, got %v %v", got, ok)
	}
	if _, ok := AsAppError(errors.New("plain")); ok {
		t.Fatal("plain error should not convert")
	}
}

func TestToEnvelope_NilDetailsBecomeEmptySlice(t *testing.T) {
	err := &AppError{
		HTTPStatus: http.StatusBadRequest,
		Code:       CodeValidationError,
		Message:    "bad",
		Details:    nil,
	}
	env := ToEnvelope(err)
	if env.Error.Details == nil || len(env.Error.Details) != 0 {
		t.Fatalf("details should be empty slice, got %#v", env.Error.Details)
	}
}
