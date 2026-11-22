package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/config"
)

func setEnvironment(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if ok {
			t.Setenv(name, value)
		}
	}
}

func TestLoad(t *testing.T) {
	setEnvironment(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Telemetry.ExportTimeout != 2*time.Second || cfg.Telemetry.MetricInterval != 5*time.Second || cfg.Telemetry.ServiceVersion == "" {
		t.Fatalf("unexpected telemetry config: %+v", cfg.Telemetry)
	}
}

func TestLoadInvalidTelemetry(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "SERVICE_VERSION"},
		{name: "OTEL_EXPORTER_OTLP_TIMEOUT", value: "0"},
		{name: "OTEL_METRIC_EXPORT_INTERVAL", value: "-1"},
		{name: "OTEL_BSP_SCHEDULE_DELAY", value: "2s"},
		{name: "OTEL_TRACES_SAMPLER", value: "unsupported"},
		{name: "OTEL_TRACES_SAMPLER_ARG", value: "NaN"},
		{name: "OTEL_TRACES_SAMPLER_ARG", value: "1.1"},
		{name: "OTEL_EXPORTER_OTLP_PROTOCOL", value: "ftp"},
		{name: "OTEL_EXPORTER_OTLP_ENDPOINT", value: "ftp://localhost:4318"},
		{name: "ELASTIC_ENDPOINT", value: "not a URL"},
		{name: "ELASTIC_TIMEOUT", value: "0s"},
	} {
		t.Run(test.name+test.value, func(t *testing.T) {
			setEnvironment(t)
			t.Setenv(test.name, test.value)
			_, err := config.Load()
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
