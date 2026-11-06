package postgres

import (
	"context"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/infra/postgres/transaction"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tripRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewTripRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *tripRepository {
	return &tripRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *tripRepository) CountActive(ctx context.Context) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	var count int64
	err := transaction.GetExecutor(queryCtx, r.pool).QueryRow(queryCtx, "SELECT count(*) FROM trips WHERE status = $1", models.TripStatusActive).Scan(&count)
	return count, err
}

func (r *tripRepository) Create(ctx context.Context, trip *models.Trip) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	_, err := transaction.GetExecutor(queryCtx, r.pool).Exec(queryCtx, `INSERT INTO trips (
			id, user_id, driver_id,
			start_latitude, start_longitude,
			end_latitude, end_longitude,
			price, status, started_at, finished_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, trip.ID, trip.UserID, trip.DriverID, trip.StartPoint.Latitude, trip.StartPoint.Longitude, trip.EndPoint.Latitude, trip.EndPoint.Longitude, trip.Price, trip.Status, trip.StartedAt, trip.FinishedAt)
	return err
}

func (r *tripRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Trip, error) {
	return r.getByID(ctx, id, false)
}

func (r *tripRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Trip, error) {
	return r.getByID(ctx, id, true)
}

func (r *tripRepository) getByID(ctx context.Context, id uuid.UUID, forUpdate bool) (*models.Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query := `SELECT id, user_id, driver_id,
			start_latitude, start_longitude,
			end_latitude, end_longitude,
			price, status, started_at, finished_at
		 FROM trips
		 WHERE id = $1`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var trip models.Trip
	err := transaction.GetExecutor(queryCtx, r.pool).QueryRow(queryCtx, query, id).Scan(&trip.ID, &trip.UserID, &trip.DriverID, &trip.StartPoint.Latitude, &trip.StartPoint.Longitude, &trip.EndPoint.Latitude, &trip.EndPoint.Longitude, &trip.Price, &trip.Status, &trip.StartedAt, &trip.FinishedAt)
	if err != nil {
		return nil, err
	}

	return &trip, nil
}

func (r *tripRepository) Update(ctx context.Context, trip *models.Trip) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	err := transaction.GetExecutor(queryCtx, r.pool).QueryRow(queryCtx, `UPDATE trips
		 SET status = $1, finished_at = $2, updated_at = now()
		 WHERE id = $3
		 RETURNING id`, trip.Status, trip.FinishedAt, trip.ID).Scan(&trip.ID)
	return err
}
