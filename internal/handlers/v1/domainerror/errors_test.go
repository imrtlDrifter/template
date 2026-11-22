package domainerror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/imrtlDrifter/template/internal/domainerrors"
	api "github.com/imrtlDrifter/template/internal/generated"
	httpres "github.com/imrtlDrifter/template/pkg/http_res"

	"github.com/gin-gonic/gin"
)

func TestWrapTripError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   string
		detail string
		status int
	}{
		{
			name:   "empty body",
			err:    domainerrors.ErrEmptyRequestBody,
			status: http.StatusBadRequest,
			code:   "invalid_request",
			detail: "Request body is required",
		},
		{
			name:   "trip not found",
			err:    domainerrors.ErrTripNotFound,
			status: http.StatusNotFound,
			code:   "trip_not_found",
			detail: "Trip was not found",
		},
		{
			name:   "completed trip",
			err:    domainerrors.ErrTripCompleted,
			status: http.StatusConflict,
			code:   "trip_completed",
			detail: "Operation is not allowed for a completed trip",
		},
		{
			name:   "busy driver",
			err:    domainerrors.ErrDriverBusy,
			status: http.StatusConflict,
			code:   "driver_busy",
			detail: "Driver already has an active trip",
		},
		{
			name:   "idempotency conflict",
			err:    domainerrors.ErrIdempotencyConflict,
			status: http.StatusConflict,
			code:   "idempotency_conflict",
			detail: "Idempotency-Key was already used with a different request body",
		},
		{
			name:   "validation",
			err:    &domainerrors.ValidationError{Field: "price", Message: "must be greater than or equal to 0"},
			status: http.StatusBadRequest,
			code:   "invalid_request",
			detail: "field price: must be greater than or equal to 0",
		},
		{
			name:   "internal error",
			err:    errors.New("secret database error"),
			status: http.StatusInternalServerError,
			code:   "internal_error",
			detail: "Internal server error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrappedErr := WrapTripError(fmt.Errorf("operation failed: %w", test.err))
			if !errors.Is(wrappedErr, test.err) {
				t.Fatalf("wrapper lost original error: %v", wrappedErr)
			}

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/trips", nil)
			httpres.HandleDomainError(ctx, wrappedErr)

			if recorder.Code != test.status {
				t.Fatalf("status = %d, expected %d", recorder.Code, test.status)
			}
			if recorder.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("unexpected Content-Type: %q", recorder.Header().Get("Content-Type"))
			}
			var problem api.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Code != test.code || int(problem.Status) != test.status || problem.Detail == nil || *problem.Detail != test.detail {
				t.Fatalf("unexpected problem: %+v", problem)
			}
		})
	}
}

func TestWrapValidationError(t *testing.T) {
	validationErr := &domainerrors.ValidationError{Field: "price", Message: "must be non-negative"}
	if !errors.Is(WrapValidationError(validationErr), validationErr) {
		t.Fatal("wrapper lost the validation error")
	}

	otherErr := errors.New("unrelated error")
	if WrapValidationError(otherErr) != otherErr {
		t.Fatal("wrapper changed an unrelated error")
	}
	if WrapValidationError(nil) != nil || WrapTripError(nil) != nil {
		t.Fatal("wrapper changed nil error")
	}
}
