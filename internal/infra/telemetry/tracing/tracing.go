package tracing

import (
	"context"
	"fmt"
	"strings"

	"github.com/imrtlDrifter/template/internal/config"

	"go.opentelemetry.io/contrib/propagators/b3"
	jaegerprop "go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracing(ctx context.Context, cfg config.Telemetry) (*tracesdk.TracerProvider, error) {
	res, err := NewResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create trace resource: %w", err)
	}
	options := []tracesdk.TracerProviderOption{tracesdk.WithResource(res), tracesdk.WithSampler(newSampler(cfg))}
	if cfg.Endpoint != "" {
		var exporter *otlptrace.Exporter
		var exportErr error
		if cfg.Protocol == "grpc" {
			exporter, exportErr = otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(cfg.Endpoint), otlptracegrpc.WithTimeout(cfg.ExportTimeout))
		} else {
			exporter, exportErr = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(strings.TrimRight(cfg.Endpoint, "/")+"/v1/traces"), otlptracehttp.WithTimeout(cfg.ExportTimeout))
		}
		if exportErr != nil {
			return nil, fmt.Errorf("create trace exporter: %w", exportErr)
		}
		options = append(options, tracesdk.WithBatcher(exporter, tracesdk.WithBatchTimeout(cfg.TraceBatchTimeout), tracesdk.WithExportTimeout(cfg.ExportTimeout)))
	}
	return tracesdk.NewTracerProvider(options...), nil
}

func NewResource(ctx context.Context, cfg config.Telemetry) (*resource.Resource, error) {
	return resource.New(ctx, resource.WithFromEnv(), resource.WithAttributes(attribute.String("service.name", cfg.ServiceName), attribute.String("service.version", cfg.ServiceVersion)))
}

func NewPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(jaegerprop.Jaeger{}, propagation.TraceContext{}, b3.New(), propagation.Baggage{})
}

func newSampler(cfg config.Telemetry) tracesdk.Sampler {
	switch cfg.Sampler {
	case "always_off":
		return tracesdk.NeverSample()
	case "always_on":
		return tracesdk.AlwaysSample()
	case "parentbased_always_off":
		return tracesdk.ParentBased(tracesdk.NeverSample())
	case "traceidratio":
		return tracesdk.TraceIDRatioBased(cfg.SamplerRatio)
	case "parentbased_traceidratio":
		return tracesdk.ParentBased(tracesdk.TraceIDRatioBased(cfg.SamplerRatio))
	default:
		return tracesdk.ParentBased(tracesdk.AlwaysSample())
	}
}
