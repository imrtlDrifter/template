package postgres

import (
	"context"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/infra/postgres/transaction"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type idempotencyRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewIdempotencyRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *idempotencyRepository {
	return &idempotencyRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *idempotencyRepository) TryCreate(ctx context.Context, key uuid.UUID, requestHash []byte, expiresAt time.Time) (bool, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	tag, err := transaction.GetExecutor(queryCtx, r.pool).Exec(queryCtx, `INSERT INTO idempotency_keys (key, request_hash, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO NOTHING`, key, requestHash, expiresAt)
	if err != nil {
		return false, err
	}

	return tag.RowsAffected() == 1, nil
}

func (r *idempotencyRepository) Get(ctx context.Context, key uuid.UUID) (*models.IdempotencyRecord, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var record models.IdempotencyRecord
	err := transaction.GetExecutor(queryCtx, r.pool).QueryRow(queryCtx, `SELECT key, request_hash, trip_id, expires_at
		 FROM idempotency_keys
		 WHERE key = $1`, key).Scan(&record.Key, &record.RequestHash, &record.TripID, &record.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (r *idempotencyRepository) BindTrip(ctx context.Context, key, tripID uuid.UUID) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	_, err := transaction.GetExecutor(queryCtx, r.pool).Exec(queryCtx, `UPDATE idempotency_keys
		 SET trip_id = $1
		 WHERE key = $2`, tripID, key)

	return err
}

func (r *idempotencyRepository) DeleteExpired(ctx context.Context) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	_, err := transaction.GetExecutor(queryCtx, r.pool).Exec(queryCtx, `DELETE FROM idempotency_keys
		 WHERE expires_at <= now()`)

	return err
}
