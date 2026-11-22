package positions

import (
	"context"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
)

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE} -exclude_interfaces ticker,positionMetrics

type (
	tripRepository interface {
		ListStaleTrips(ctx context.Context, at time.Time, staleAfter time.Duration, after *models.StaleTrip, limit int) ([]models.StaleTrip, error)
		StillStale(ctx context.Context, trip models.StaleTrip) (bool, error)
	}

	pushGateway interface {
		RequestPosition(ctx context.Context, trip models.StaleTrip) error
	}

	ticker interface {
		C() <-chan time.Time
		Stop()
	}

	positionMetrics interface {
		StaleCheck(ctx context.Context, result string)
		RowsRead(ctx context.Context, count int)
		QueueSize(ctx context.Context, size int)
		WorkersActive(ctx context.Context, count int)
		PositionRequest(ctx context.Context, result string)
		PushDuration(ctx context.Context, result string, duration time.Duration)
	}

	workerLogger interface {
		WarnContext(ctx context.Context, err error, msg string, extra ...any)
	}
)
