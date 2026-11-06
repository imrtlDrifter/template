package postgres_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/imrtlDrifter/template/internal/repositories/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRepositoryErrorsIsNotFoundError(t *testing.T) {
	tests := []struct {
		err  error
		name string
		want bool
	}{
		{name: "nil"},
		{name: "not found", err: pgx.ErrNoRows, want: true},
		{name: "wrapped not found", err: fmt.Errorf("get trip: %w", pgx.ErrNoRows), want: true},
		{name: "same text", err: errors.New(pgx.ErrNoRows.Error())},
	}

	checker := postgres.NewRepositoryErrors()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := checker.IsNotFoundError(test.err); got != test.want {
				t.Fatalf("IsNotFoundError() = %v, expected %v", got, test.want)
			}
		})
	}
}

func TestRepositoryErrorsIsDriverBusyError(t *testing.T) {
	busyErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "trips_one_active_per_driver_idx",
	}
	tests := []struct {
		err  error
		name string
		want bool
	}{
		{name: "nil"},
		{name: "driver busy", err: busyErr, want: true},
		{name: "wrapped driver busy", err: fmt.Errorf("create trip: %w", busyErr), want: true},
		{name: "another unique constraint", err: &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "trips_pkey",
		}},
		{name: "another error code", err: &pgconn.PgError{
			Code:           "23514",
			ConstraintName: "trips_one_active_per_driver_idx",
		}},
		{name: "another error", err: pgx.ErrNoRows},
	}

	checker := postgres.NewRepositoryErrors()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := checker.IsDriverBusyError(test.err); got != test.want {
				t.Fatalf("IsDriverBusyError() = %v, expected %v", got, test.want)
			}
		})
	}
}
