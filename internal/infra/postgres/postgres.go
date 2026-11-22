package postgres

import (
	"fmt"

	"github.com/imrtlDrifter/template/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewConfig(pg config.PostgreSQL) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(pg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.MaxConns = pg.MaxConns
	cfg.MinConns = pg.MinConns
	cfg.MaxConnLifetime = pg.MaxConnLifetime
	cfg.ConnConfig.ConnectTimeout = pg.ConnectTimeout

	return cfg, nil
}
