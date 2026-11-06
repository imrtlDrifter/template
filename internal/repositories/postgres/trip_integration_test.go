//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
	"github.com/imrtlDrifter/template/internal/infra/postgres/transaction"
	"github.com/imrtlDrifter/template/internal/infra/telemetry"
	"github.com/imrtlDrifter/template/internal/repositories/postgres"
	createservice "github.com/imrtlDrifter/template/internal/service/trip/create"
	finishservice "github.com/imrtlDrifter/template/internal/service/trip/finish"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestFinishConcurrent(t *testing.T) {
	pool := newTestPool(t)
	tripRepo := postgres.NewTripRepository(pool, 3*time.Second)
	historyRepo := postgres.NewStatusHistoryRepository(pool, 3*time.Second)
	idempotencyRepo := postgres.NewIdempotencyRepository(pool, 3*time.Second)
	txManager := transaction.NewTransactionManager(pool, 3*time.Second)
	repositoryErrors := postgres.NewRepositoryErrors()
	metrics, err := telemetry.NewMetrics(noop.NewMeterProvider(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	create := createservice.NewTripService(tripRepo, historyRepo, idempotencyRepo, txManager, repositoryErrors, metrics)
	finish := finishservice.NewTripService(tripRepo, historyRepo, txManager, repositoryErrors, metrics)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	command := &models.CreateTripCommand{UserID: uuid.New(), DriverID: uuid.New(), Price: 100}
	trip, created, err := create.Create(ctx, command, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	t.Cleanup(func() { deleteTestTrip(t, pool, trip.ID) })
	if !created {
		t.Fatal("create did not create a new trip")
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, finishErr := finish.Finish(ctx, trip.ID)
			results <- finishErr
		}()
	}
	close(start)

	var successCount, completedCount int
	for range 2 {
		select {
		case finishErr := <-results:
			switch {
			case finishErr == nil:
				successCount++
			case errors.Is(finishErr, domainerrors.ErrTripCompleted):
				completedCount++
			default:
				t.Fatalf("finish trip: %v", finishErr)
			}
		case <-ctx.Done():
			t.Fatal("concurrent finish timed out")
		}
	}
	if successCount != 1 || completedCount != 1 {
		t.Fatalf("successes = %d, completed errors = %d; expected one of each", successCount, completedCount)
	}

	stored, err := tripRepo.GetByID(ctx, trip.ID)
	if err != nil {
		t.Fatalf("get completed trip: %v", err)
	}
	if stored.Status != models.TripStatusCompleted || stored.FinishedAt == nil {
		t.Fatalf("unexpected stored trip: %+v", stored)
	}

	var activeCount, completedHistoryCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE to_status = 'active'), count(*) FILTER (WHERE from_status = 'active' AND to_status = 'completed') FROM trip_status_history WHERE trip_id = $1", trip.ID).Scan(&activeCount, &completedHistoryCount)
	if err != nil {
		t.Fatalf("count status history: %v", err)
	}
	if activeCount != 1 || completedHistoryCount != 1 {
		t.Fatalf("active history = %d, completed history = %d; expected one of each", activeCount, completedHistoryCount)
	}
}

func TestTransactionRollsBackBothTables(t *testing.T) {
	pool := newTestPool(t)
	tripRepo := postgres.NewTripRepository(pool, 3*time.Second)
	historyRepo := postgres.NewStatusHistoryRepository(pool, 3*time.Second)
	txManager := transaction.NewTransactionManager(pool, 3*time.Second)
	repositoryErrors := postgres.NewRepositoryErrors()
	trip := &models.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(), Price: 100, Status: models.TripStatusActive, StartedAt: time.Now().UTC()}
	t.Cleanup(func() { deleteTestTrip(t, pool, trip.ID) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := txManager.Do(ctx, func(ctx context.Context) error {
		if err := tripRepo.Create(ctx, trip); err != nil {
			return err
		}
		if err := historyRepo.CreateStatusHistory(ctx, &models.TripStatusHistory{TripID: trip.ID, ToStatus: models.TripStatusActive}); err != nil {
			return err
		}
		return historyRepo.CreateStatusHistory(ctx, &models.TripStatusHistory{TripID: trip.ID, ToStatus: models.TripStatus("invalid")})
	})
	if err == nil {
		t.Fatal("expected a constraint violation")
	}

	if _, err := tripRepo.GetByID(ctx, trip.ID); !repositoryErrors.IsNotFoundError(err) {
		t.Fatalf("trip was not rolled back: %v", err)
	}
	var historyCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM trip_status_history WHERE trip_id = $1", trip.ID).Scan(&historyCount); err != nil {
		t.Fatalf("count status history: %v", err)
	}
	if historyCount != 0 {
		t.Fatalf("history was not rolled back: count = %d", historyCount)
	}
}

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create test pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	return pool
}

func deleteTestTrip(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", id); err != nil {
		t.Errorf("delete test trip: %v", err)
	}
}
