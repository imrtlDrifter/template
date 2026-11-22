package postgres

import (
	"context"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/infra/postgres/transaction"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *tripRepository) CreatePosition(ctx context.Context, position *models.TripPosition) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	return transaction.GetExecutor(queryCtx, r.pool).QueryRow(queryCtx, `INSERT INTO trip_positions
		(trip_id, latitude, longitude, recorded_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, recorded_at`, position.TripID, position.Coordinates.Latitude, position.Coordinates.Longitude, position.RecordedAt).Scan(&position.ID, &position.RecordedAt)
}

func (r *tripRepository) UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	_, err := transaction.GetExecutor(queryCtx, r.pool).Exec(queryCtx, `UPDATE trips
		SET last_position_at = $2, updated_at = now()
		WHERE id = $1 AND (last_position_at IS NULL OR last_position_at < $2)`, tripID, recordedAt)
	return err
}

func (r *tripRepository) ListPositions(ctx context.Context, tripID uuid.UUID) ([]models.TripPosition, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := transaction.GetExecutor(queryCtx, r.pool).Query(queryCtx, `SELECT id, latitude, longitude, recorded_at
		FROM trip_positions WHERE trip_id = $1 ORDER BY recorded_at, id`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []models.TripPosition
	for rows.Next() {
		position := models.TripPosition{TripID: tripID}
		if err := rows.Scan(&position.ID, &position.Coordinates.Latitude, &position.Coordinates.Longitude, &position.RecordedAt); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	return positions, rows.Err()
}

func (r *tripRepository) ListStaleTrips(ctx context.Context, at time.Time, staleAfter time.Duration, after *models.StaleTrip, limit int) ([]models.StaleTrip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	since := pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}
	afterID := uuid.Nil
	if after != nil {
		since = pgtype.Timestamptz{Time: after.Since, Valid: true}
		afterID = after.ID
	}

	interval := pgtype.Interval{Microseconds: staleAfter.Microseconds(), Valid: true}
	rows, err := transaction.GetExecutor(queryCtx, r.pool).Query(queryCtx, `SELECT id, driver_id, COALESCE(last_position_at, started_at) AS since
		FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at) < $1::timestamptz - $2::interval
		  AND (COALESCE(last_position_at, started_at), id) > ($3, $4)
		ORDER BY since, id
		LIMIT $5`, at, interval, since, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trips := make([]models.StaleTrip, 0, limit)
	for rows.Next() {
		var trip models.StaleTrip
		if err := rows.Scan(&trip.ID, &trip.DriverID, &trip.Since); err != nil {
			return nil, err
		}
		trips = append(trips, trip)
	}
	return trips, rows.Err()
}

func (r *tripRepository) StillStale(ctx context.Context, trip models.StaleTrip) (bool, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var stale bool
	err := transaction.GetExecutor(queryCtx, r.pool).QueryRow(queryCtx, `SELECT EXISTS (
		SELECT 1 FROM trips WHERE id = $1 AND status = 'active'
		AND COALESCE(last_position_at, started_at) = $2
	)`, trip.ID, trip.Since).Scan(&stale)
	return stale, err
}
