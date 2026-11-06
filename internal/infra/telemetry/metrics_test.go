package telemetry_test

import (
	"testing"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/infra/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/mock/gomock"
)

func TestBusinessMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	ctrl := gomock.NewController(t)
	logger := NewMockbusinessLogger(ctrl)
	counter := NewMocktripCounter(ctrl)
	metrics, err := telemetry.NewMetrics(provider, logger)
	if err != nil {
		t.Fatal(err)
	}
	registration, err := metrics.ObserveActiveTrips(counter)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registration.Unregister(); err != nil {
			t.Error(err)
		}
		if err := provider.Shutdown(t.Context()); err != nil {
			t.Error(err)
		}
	})
	trip := &models.Trip{ID: uuid.New()}
	logger.EXPECT().InfoContext(t.Context(), "trip created", "trip_id", trip.ID.String())
	logger.EXPECT().InfoContext(t.Context(), "trip completed", "trip_id", trip.ID.String())
	metrics.Created(t.Context(), trip)
	metrics.Completed(t.Context(), trip)
	for _, active := range []int64{3, 2, 0, 5} {
		counter.EXPECT().CountActive(t.Context()).Return(active, nil)
		var result metricdata.ResourceMetrics
		if err := reader.Collect(t.Context(), &result); err != nil {
			t.Fatal(err)
		}
		values := make(map[string]int64)
		for _, scope := range result.ScopeMetrics {
			for _, measurement := range scope.Metrics {
				switch data := measurement.Data.(type) {
				case metricdata.Sum[int64]:
					values[measurement.Name] = data.DataPoints[0].Value
				case metricdata.Gauge[int64]:
					values[measurement.Name] = data.DataPoints[0].Value
				}
			}
		}
		if values["tripgo.trips.created"] != 1 || values["tripgo.trips.completed"] != 1 || values["tripgo.trips.active"] != active {
			t.Fatalf("unexpected business metrics: %+v", values)
		}
	}
}
