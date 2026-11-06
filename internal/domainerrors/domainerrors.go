package domainerrors

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyRequestBody      = errors.New("request body is required")
	ErrTripNotFound          = errors.New("trip not found")
	ErrTripCompleted         = errors.New("trip already completed")
	ErrDriverBusy            = errors.New("driver already has an active trip")
	ErrIdempotencyConflict   = errors.New("idempotency key was used with another request")
	ErrValidationChainBroken = errors.New("validation chain broken")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("field %s: %s", e.Field, e.Message)
}
