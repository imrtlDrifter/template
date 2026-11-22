package positions

import (
	"context"
	"errors"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"go.opentelemetry.io/otel/codes"
)

var errQueueFull = errors.New("position request queue full")

func (w *Worker) scan(ctx context.Context) {
	ticks := w.ticker(w.cfg.CheckInterval)
	defer ticks.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case at := <-ticks.C():
			w.scanTick(ctx, at)
		}
	}
}

func (w *Worker) scanTick(ctx context.Context, at time.Time) {
	ctx, span := w.tracer.Start(ctx, "positions.scan")
	defer span.End()
	var after *models.StaleTrip
	var rowsRead int
	defer func() { w.metrics.RowsRead(ctx, rowsRead) }()

	for ctx.Err() == nil {
		trips, err := w.repo.ListStaleTrips(ctx, at, w.cfg.StaleAfter, after, w.cfg.BatchSize)
		if err != nil {
			if ctx.Err() == nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "list stale trips")
				w.log.WarnContext(ctx, err, "scan stale trips failed")
			}
			return
		}
		rowsRead += len(trips)

		for _, trip := range trips {
			if ctx.Err() != nil || w.enqueue(ctx, trip) {
				return
			}
		}
		if len(trips) < w.cfg.BatchSize {
			return
		}
		after = &trips[len(trips)-1]
	}
}

func (w *Worker) enqueue(ctx context.Context, trip models.StaleTrip) bool {
	result, evicted := w.queue.enqueue(ctx, trip)
	w.metrics.StaleCheck(ctx, result)
	if evicted {
		w.metrics.StaleCheck(ctx, dropped)
	}
	if evicted || result == dropped {
		w.metrics.PositionRequest(ctx, dropped)
		w.log.WarnContext(ctx, errQueueFull, "position request dropped")
		return true
	}
	return false
}
