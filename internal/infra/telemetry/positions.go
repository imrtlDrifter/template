package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PositionMetrics struct {
	saved         metric.Int64Counter
	staleChecks   metric.Int64Counter
	rowsRead      metric.Int64Histogram
	queueSize     metric.Int64Gauge
	workersActive metric.Int64Gauge
	requests      metric.Int64Counter
	pushDuration  metric.Float64Histogram
}

func NewPositionMetrics(provider metric.MeterProvider) (*PositionMetrics, error) {
	meter := provider.Meter("trip-service")
	saved, err := meter.Int64Counter("tripgo.positions.saved")
	if err != nil {
		return nil, fmt.Errorf("create saved positions counter: %w", err)
	}
	staleChecks, err := meter.Int64Counter("tripgo.stale_checks")
	if err != nil {
		return nil, fmt.Errorf("create stale checks counter: %w", err)
	}
	rowsRead, err := meter.Int64Histogram("tripgo.stale_checks.rows_read", metric.WithExplicitBucketBoundaries(0, 10, 50, 100, 200, 500, 1000, 5000, 10000))
	if err != nil {
		return nil, fmt.Errorf("create rows read histogram: %w", err)
	}
	queueSize, err := meter.Int64Gauge("tripgo.position_requests.queue.size")
	if err != nil {
		return nil, fmt.Errorf("create queue size gauge: %w", err)
	}
	workersActive, err := meter.Int64Gauge("tripgo.position_requests.workers.active")
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}
	requests, err := meter.Int64Counter("tripgo.position_requests")
	if err != nil {
		return nil, fmt.Errorf("create position requests counter: %w", err)
	}
	pushDuration, err := meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10))
	if err != nil {
		return nil, fmt.Errorf("create push duration histogram: %w", err)
	}

	return &PositionMetrics{
		saved: saved, staleChecks: staleChecks, rowsRead: rowsRead, queueSize: queueSize,
		workersActive: workersActive, requests: requests, pushDuration: pushDuration,
	}, nil
}

func (m *PositionMetrics) PositionSaved(ctx context.Context) {
	m.saved.Add(ctx, 1)
}

func (m *PositionMetrics) StaleCheck(ctx context.Context, result string) {
	m.staleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (m *PositionMetrics) RowsRead(ctx context.Context, count int) {
	m.rowsRead.Record(ctx, int64(count))
}

func (m *PositionMetrics) QueueSize(ctx context.Context, size int) {
	m.queueSize.Record(ctx, int64(size))
}

func (m *PositionMetrics) WorkersActive(ctx context.Context, count int) {
	m.workersActive.Record(ctx, int64(count))
}

func (m *PositionMetrics) PositionRequest(ctx context.Context, result string) {
	m.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (m *PositionMetrics) PushDuration(ctx context.Context, result string, duration time.Duration) {
	m.pushDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attribute.String("result", result)))
}
