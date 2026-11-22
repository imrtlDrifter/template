//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/infra/postgres/transaction"
	"github.com/imrtlDrifter/template/internal/repositories/postgres"
	"github.com/imrtlDrifter/template/internal/service/trip/positions"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListStaleTrips(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewTripRepository(pool, 3*time.Second)
	ctx := t.Context()
	now := time.Now().Truncate(time.Second)
	old := now.Add(-24 * time.Hour)
	without := createPositionTrip(t, pool, repo, old)
	equal := createPositionTrip(t, pool, repo, old)
	older := createPositionTrip(t, pool, repo, old)
	fresh := createPositionTrip(t, pool, repo, old)
	completed := createPositionTrip(t, pool, repo, old)
	boundary := createPositionTrip(t, pool, repo, now.Add(-time.Minute))

	if err := repo.UpdateLastPositionAt(ctx, older.ID, old.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateLastPositionAt(ctx, fresh.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := completed.Finish(now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, completed); err != nil {
		t.Fatal(err)
	}

	var got []uuid.UUID
	var cursor *models.StaleTrip
	for {
		page, err := repo.ListStaleTrips(ctx, now, time.Minute, cursor, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		cursor = &page[0]
		switch cursor.ID {
		case fresh.ID, completed.ID, boundary.ID:
			t.Fatalf("ineligible trip %s", cursor.ID)
		case older.ID, equal.ID, without.ID:
			got = append(got, cursor.ID)
		}
	}

	ties := []string{without.ID.String(), equal.ID.String()}
	slices.Sort(ties)
	want := []uuid.UUID{older.ID, uuid.MustParse(ties[0]), uuid.MustParse(ties[1])}
	if !slices.Equal(got, want) {
		t.Fatalf("stale trips = %v, want %v", got, want)
	}
}

func TestSavePositionTransaction(t *testing.T) {
	pool := newTestPool(t)
	repo := postgres.NewTripRepository(pool, 3*time.Second)
	tx := transaction.NewTransactionManager(pool, 3*time.Second)
	now := time.Now().UTC().Truncate(time.Second)
	trip := createPositionTrip(t, pool, repo, now.Add(-time.Hour))
	effects := &positionEffects{}
	service := positions.NewTripService(repo, tx, postgres.NewRepositoryErrors(), effects, effects, func() time.Time { return now })
	ctx := t.Context()

	inserted := make([]models.TripPosition, 0, 3)
	for _, at := range []time.Time{now, now.Add(-time.Minute), now} {
		position := models.TripPosition{TripID: trip.ID, RecordedAt: at}
		if err := service.Create(ctx, &position); err != nil {
			t.Fatal(err)
		}
		inserted = append(inserted, position)
	}

	failed := errors.New("fail after writing both tables")
	failing := failingPositionRepo{positionRepo: repo, err: failed}
	service = positions.NewTripService(failing, tx, postgres.NewRepositoryErrors(), effects, effects, func() time.Time { return now })
	position := &models.TripPosition{TripID: trip.ID, RecordedAt: now.Add(time.Second)}
	if err := service.Create(ctx, position); !errors.Is(err, failed) {
		t.Fatalf("create = %v", err)
	}

	stored, err := repo.GetByID(ctx, trip.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.LastPositionAt == nil || !stored.LastPositionAt.Equal(now) {
		t.Fatalf("last_position_at = %v, want %v", stored.LastPositionAt, now)
	}
	points, err := service.List(ctx, trip.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 || points[0].ID != inserted[1].ID || points[1].ID != inserted[0].ID || points[2].ID != inserted[2].ID {
		t.Fatalf("unexpected persisted points: %v", points)
	}
	if effects.saved != 3 || effects.canceled != 3 {
		t.Fatalf("post-commit effects ran %d and %d times", effects.saved, effects.canceled)
	}
}

type tripCreator interface {
	Create(ctx context.Context, trip *models.Trip) error
}

func createPositionTrip(t *testing.T, pool *pgxpool.Pool, repo tripCreator, startedAt time.Time) *models.Trip {
	t.Helper()
	trip := &models.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(), Status: models.TripStatusActive, StartedAt: startedAt}
	if err := repo.Create(t.Context(), trip); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deleteTestTrip(t, pool, trip.ID) })
	return trip
}

type positionRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Trip, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Trip, error)
	CreatePosition(ctx context.Context, position *models.TripPosition) error
	UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
	ListPositions(ctx context.Context, tripID uuid.UUID) ([]models.TripPosition, error)
}

type failingPositionRepo struct {
	positionRepo
	err error
}

func (r failingPositionRepo) UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	if err := r.positionRepo.UpdateLastPositionAt(ctx, tripID, recordedAt); err != nil {
		return err
	}
	return r.err
}

type positionEffects struct {
	saved    int
	canceled int
}

func (e *positionEffects) PositionSaved(context.Context) { e.saved++ }

func (e *positionEffects) Cancel(context.Context, uuid.UUID) { e.canceled++ }
