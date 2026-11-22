package telemetry_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/imrtlDrifter/template/internal/generated"
	"github.com/imrtlDrifter/template/internal/handlers/middlewares"
	"github.com/imrtlDrifter/template/internal/infra/telemetry/logger"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	metricSDK "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	traceSDK "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMiddleware(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		panic  bool
	}{
		{name: "success", status: http.StatusOK},
		{name: "client error", status: http.StatusBadRequest},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "panic", status: http.StatusInternalServerError, panic: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			spans := tracetest.NewSpanRecorder()
			tracerProvider := traceSDK.NewTracerProvider(traceSDK.WithSpanProcessor(spans))
			reader := metricSDK.NewManualReader()
			provider := metricSDK.NewMeterProvider(metricSDK.WithReader(reader))
			t.Cleanup(func() {
				if err := provider.Shutdown(t.Context()); err != nil {
					t.Error(err)
				}
			})
			meter := provider.Meter("test")
			duration, err := meter.Float64Histogram("http.server.request.duration")
			if err != nil {
				t.Fatal(err)
			}
			active, err := meter.Int64UpDownCounter("http.server.active_requests")
			if err != nil {
				t.Fatal(err)
			}
			var logs bytes.Buffer
			log := logger.NewLogger(&logs, logger.Config{Service: "test", Level: slog.LevelInfo}, nil)
			router := gin.New()
			router.Use(middlewares.NewRecovery(log).Middleware, middlewares.NewTracing(tracerProvider.Tracer("test"), propagation.TraceContext{}).Middleware, log.GinMiddleware, (middlewares.Metrics{RequestDuration: duration, ActiveRequests: active}).Middleware)
			router.GET("/trips/:id", func(c *gin.Context) {
				if test.panic {
					panic("secret panic")
				}
				c.Status(test.status)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/trips/123", nil))
			if recorder.Code != test.status {
				t.Fatalf("status = %d", recorder.Code)
			}
			if test.panic {
				var problem api.Problem
				if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != "internal_error" || strings.Contains(recorder.Body.String(), "secret") {
					t.Fatalf("unexpected recovery body: %s", recorder.Body.String())
				}
			}
			ended := spans.Ended()
			if len(ended) != 1 || ended[0].Name() != "GET /trips/:id" {
				t.Fatalf("unexpected spans: %v", ended)
			}
			if test.status >= http.StatusInternalServerError && ended[0].Status().Code != codes.Error {
				t.Fatal("server error span is not marked as error")
			}
			traceID := ended[0].SpanContext().TraceID().String()
			if strings.Count(logs.String(), `"msg":"request handled"`) != 1 || !strings.Contains(logs.String(), traceID) || !strings.Contains(logs.String(), `"route":"/trips/:id"`) {
				t.Fatalf("unexpected logs: %s", logs.String())
			}
			var data metricdata.ResourceMetrics
			if err := reader.Collect(t.Context(), &data); err != nil {
				t.Fatal(err)
			}
			assertHTTPMetrics(t, data, int64(test.status))
		})
	}
}

func assertHTTPMetrics(t *testing.T, data metricdata.ResourceMetrics, status int64) {
	t.Helper()
	var foundDuration, foundActive bool
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			switch value := measurement.Data.(type) {
			case metricdata.Histogram[float64]:
				if len(value.DataPoints) != 1 {
					t.Fatalf("unexpected durations: %+v", value)
				}
				point := value.DataPoints[0]
				route, _ := point.Attributes.Value("http.route")
				code, _ := point.Attributes.Value("http.response.status_code")
				if point.Count != 1 || route.AsString() != "/trips/:id" || code.AsInt64() != status {
					t.Fatalf("unexpected duration attributes: %+v", point)
				}
				foundDuration = true
			case metricdata.Sum[int64]:
				if len(value.DataPoints) != 1 || value.DataPoints[0].Value != 0 {
					t.Fatalf("active requests were not decremented: %+v", value)
				}
				foundActive = true
			}
		}
	}
	if !foundDuration || !foundActive {
		t.Fatal("HTTP metrics missing")
	}
}
