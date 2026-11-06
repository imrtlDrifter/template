package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Postgres     *pgxpool.Pool
	queryTimeout time.Duration
}

func NewDatabase(pool *pgxpool.Pool, queryTimeout time.Duration) *Database {
	return &Database{Postgres: pool, queryTimeout: queryTimeout}
}

func (d *Database) CheckReady(ctx context.Context) error {
	readyCtx, cancel := context.WithTimeout(ctx, d.queryTimeout)
	defer cancel()

	err := d.Postgres.Ping(readyCtx)
	if err != nil {
		return fmt.Errorf("postgres connection not ready: %w", err)
	}

	return nil
}

func (d *Database) Close() {
	if d == nil || d.Postgres == nil {
		return
	}

	d.Postgres.Close()
}
