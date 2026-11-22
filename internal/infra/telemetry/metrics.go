package telemetry

import (
	"context"
	"fmt"
	"strings"

	"github.com/imrtlDrifter/template/internal/config"
	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/infra/telemetry/tracing"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	metricSDK "go.opentelemetry.io/otel/sdk/metric"
)

type Metrics struct {
	logger          businessLogger
	meter           metric.Meter
	RequestDuration metric.Float64Histogram
	ActiveRequests  metric.Int64UpDownCounter
	DBDuration      metric.Float64Histogram
	TripsCreated    metric.Int64Counter
	TripsCompleted  metric.Int64Counter
	ActiveTrips     metric.Int64ObservableGauge
}

func InitMetrics(ctx context.Context, cfg config.Telemetry) (*metricSDK.MeterProvider, error) {
	res, resourceErr := tracing.NewResource(ctx, cfg)
	if resourceErr != nil {
		return nil, fmt.Errorf("create metric resource: %w", resourceErr)
	}
	opts := []metricSDK.Option{metricSDK.WithResource(res)}
	if cfg.Endpoint != "" {
		var exporter metricSDK.Exporter
		var err error
		if cfg.Protocol == "grpc" {
			exporter, err = otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(cfg.Endpoint), otlpmetricgrpc.WithTimeout(cfg.ExportTimeout))
		} else {
			exporter, err = otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(strings.TrimRight(cfg.Endpoint, "/")+"/v1/metrics"), otlpmetrichttp.WithTimeout(cfg.ExportTimeout))
		}
		if err != nil {
			return nil, fmt.Errorf("create metric exporter: %w", err)
		}
		opts = append(opts, metricSDK.WithReader(metricSDK.NewPeriodicReader(exporter, metricSDK.WithInterval(cfg.MetricInterval), metricSDK.WithTimeout(cfg.ExportTimeout))))
	}
	provider := metricSDK.NewMeterProvider(opts...)
	return provider, nil
}

func NewMetrics(provider metric.MeterProvider, logger businessLogger) (*Metrics, error) {
	meter := provider.Meter("trip-service")
	requestDuration, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10))
	if err != nil {
		return nil, fmt.Errorf("create HTTP duration metric: %w", err)
	}
	activeRequests, err := meter.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, fmt.Errorf("create HTTP active metric: %w", err)
	}
	dbDuration, err := meter.Float64Histogram("db.client.operation.duration", metric.WithUnit("s"), metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5))
	if err != nil {
		return nil, fmt.Errorf("create DB duration metric: %w", err)
	}
	created, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create trip counter: %w", err)
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create completed trip counter: %w", err)
	}
	activeTrips, err := meter.Int64ObservableGauge("tripgo.trips.active", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create active trip gauge: %w", err)
	}
	return &Metrics{logger: logger, meter: meter, RequestDuration: requestDuration, ActiveRequests: activeRequests, DBDuration: dbDuration, TripsCreated: created, TripsCompleted: completed, ActiveTrips: activeTrips}, nil
}

func (m *Metrics) ObserveActiveTrips(counter tripCounter) (metric.Registration, error) {
	return m.meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		count, err := counter.CountActive(ctx)
		if err != nil {
			return fmt.Errorf("count active trips: %w", err)
		}
		observer.ObserveInt64(m.ActiveTrips, count)
		return nil
	}, m.ActiveTrips)
}

func (m *Metrics) Created(ctx context.Context, trip *models.Trip) {
	m.TripsCreated.Add(ctx, 1)
	m.logger.InfoContext(ctx, "trip created", "trip_id", trip.ID.String())
}

func (m *Metrics) Completed(ctx context.Context, trip *models.Trip) {
	m.TripsCompleted.Add(ctx, 1)
	m.logger.InfoContext(ctx, "trip completed", "trip_id", trip.ID.String())
}
