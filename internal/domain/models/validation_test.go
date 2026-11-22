package models_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
)

const validDecision = "valid"

func TestChainStopsOnError(t *testing.T) {
	validationErr := &domainerrors.ValidationError{Field: "user_id", Message: "is required"}
	command := &models.CreateTripCommand{}
	var calls []string

	check := models.Chain(
		func(c *models.CreateTripCommand) (string, bool, error) {
			if c != command {
				t.Fatal("chain passed another command")
			}
			calls = append(calls, "first")
			return "", false, nil
		},
		func(*models.CreateTripCommand) (string, bool, error) {
			calls = append(calls, "second")
			return "", false, validationErr
		},
		func(*models.CreateTripCommand) (string, bool, error) {
			t.Fatal("chain continued after validation error")
			return validDecision, true, nil
		},
	)

	decision, handled, err := check(command)
	if !errors.Is(err, validationErr) || handled || decision != "" {
		t.Fatalf("unexpected chain result: %q, %v, %v", decision, handled, err)
	}
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("unexpected check order: %v", calls)
	}
}

func TestChainStopsWhenHandled(t *testing.T) {
	check := models.Chain(
		func(*models.CreateTripCommand) (string, bool, error) {
			return validDecision, true, nil
		},
		func(*models.CreateTripCommand) (string, bool, error) {
			t.Fatal("chain continued after handled result")
			return "", false, nil
		},
	)

	decision, handled, err := check(&models.CreateTripCommand{})
	if err != nil || !handled || decision != validDecision {
		t.Fatalf("unexpected chain result: %q, %v, %v", decision, handled, err)
	}
}

func TestChainWithoutHandler(t *testing.T) {
	tests := []struct {
		name   string
		checks []models.Check
	}{
		{name: "empty chain"},
		{name: "no terminal check", checks: []models.Check{
			func(*models.CreateTripCommand) (string, bool, error) {
				return "", false, nil
			},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, handled, err := models.Chain(test.checks...)(&models.CreateTripCommand{})
			if !errors.Is(err, domainerrors.ErrValidationChainBroken) || handled || decision != "" {
				t.Fatalf("unexpected chain result: %q, %v, %v", decision, handled, err)
			}
		})
	}
}
