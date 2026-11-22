package positions

import (
	"context"
	"sync"
	"time"

	"github.com/imrtlDrifter/template/internal/config"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	queued    = "queued"
	duplicate = "duplicate"
	dropped   = "dropped"
	success   = "success"
	failed    = "error"
)

type Worker struct {
	repo    tripRepository
	push    pushGateway
	metrics positionMetrics
	log     workerLogger
	tracer  trace.Tracer
	queue   *queue
	ticker  func(time.Duration) ticker
	cfg     config.Positions
}

func NewWorker(repo tripRepository, push pushGateway, metrics positionMetrics, log workerLogger, tracer trace.Tracer, cfg config.Positions) *Worker {
	return &Worker{
		repo: repo, push: push, metrics: metrics, log: log, tracer: tracer,
		queue: newQueue(cfg.QueueSize, metrics),
		ticker: func(interval time.Duration) ticker {
			return timeTicker{time.NewTicker(interval)}
		},
		cfg: cfg,
	}
}

type timeTicker struct{ *time.Ticker }

func (t timeTicker) C() <-chan time.Time { return t.Ticker.C }

func (w *Worker) Run(scanCtx, workCtx context.Context) {
	w.metrics.QueueSize(workCtx, 0)
	w.metrics.WorkersActive(workCtx, 0)
	var workers sync.WaitGroup
	for range w.cfg.Workers {
		workers.Go(func() { w.work(workCtx) })
	}

	w.scan(scanCtx)
	w.queue.close()
	workers.Wait()
}

func (w *Worker) Cancel(ctx context.Context, tripID uuid.UUID) {
	if w.queue.cancel(ctx, tripID) {
		w.metrics.PositionRequest(ctx, dropped)
	}
}

func (w *Worker) work(ctx context.Context) {
	for {
		t := w.queue.take(ctx)
		if t == nil {
			return
		}
		w.metrics.PositionRequest(ctx, w.process(ctx, t))
		w.queue.finish(ctx, t)
	}
}

func (w *Worker) process(ctx context.Context, task *task) string {
	if ctx.Err() != nil {
		return dropped
	}

	ctx, span := w.tracer.Start(ctx, "positions.process", trace.WithNewRoot(), trace.WithLinks(trace.Link{SpanContext: task.link}))
	defer span.End()
	taskCtx, cancel := context.WithTimeout(ctx, w.cfg.PushTimeout)
	defer cancel()
	stale, err := w.repo.StillStale(taskCtx, task.trip)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "recheck trip")
		w.log.WarnContext(ctx, err, "recheck stale trip failed")
		return failed
	}
	if !stale {
		return dropped
	}

	start := time.Now()
	err = w.push.RequestPosition(taskCtx, task.trip)
	duration := time.Since(start)
	result := success
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "request position")
		w.log.WarnContext(ctx, err, "position request failed")
		result = failed
	}

	w.metrics.PushDuration(ctx, result, duration)
	return result
}
