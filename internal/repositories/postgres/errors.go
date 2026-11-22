package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RepositoryErrors struct{}

func NewRepositoryErrors() *RepositoryErrors {
	return &RepositoryErrors{}
}

func (r *RepositoryErrors) IsNotFoundError(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func (r *RepositoryErrors) IsDriverBusyError(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_driver_idx"
}
