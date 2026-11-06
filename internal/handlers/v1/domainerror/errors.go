package domainerror

import (
	"errors"
	"net/http"

	"github.com/imrtlDrifter/template/internal/domainerrors"
	httpres "github.com/imrtlDrifter/template/pkg/http_res"
)

const (
	InvalidRequestCode  = "invalid_request"
	TripNotFoundCode    = "trip_not_found"
	TripCompletedCode   = "trip_completed"
	DriverBusyCode      = "driver_busy"
	InvalidRequestTitle = "Invalid request"
	TripNotFoundTitle   = "Trip not found"
	TripCompletedTitle  = "Trip completed"
	DriverBusyTitle     = "Driver busy"
)

func WrapValidationError(err error) error {
	var validationErr *domainerrors.ValidationError
	if !errors.As(err, &validationErr) {
		return err
	}

	return httpres.NewHTTPError(err, http.StatusBadRequest, InvalidRequestCode, InvalidRequestTitle, validationErr.Error())
}

func WrapTripError(err error) error {
	switch {
	case errors.Is(err, domainerrors.ErrEmptyRequestBody):
		return httpres.NewHTTPError(err, http.StatusBadRequest, InvalidRequestCode, InvalidRequestTitle, "Request body is required")
	case errors.Is(err, domainerrors.ErrTripNotFound):
		return httpres.NewHTTPError(err, http.StatusNotFound, TripNotFoundCode, TripNotFoundTitle, "Trip was not found")
	case errors.Is(err, domainerrors.ErrTripCompleted):
		return httpres.NewHTTPError(err, http.StatusConflict, TripCompletedCode, TripCompletedTitle, "Operation is not allowed for a completed trip")
	case errors.Is(err, domainerrors.ErrDriverBusy):
		return httpres.NewHTTPError(err, http.StatusConflict, DriverBusyCode, DriverBusyTitle, "Driver already has an active trip")
	case errors.Is(err, domainerrors.ErrIdempotencyConflict):
		return httpres.NewHTTPError(err, http.StatusConflict, "idempotency_conflict", "Idempotency conflict", "Idempotency-Key was already used with a different request body")
	default:
		return WrapValidationError(err)
	}
}
