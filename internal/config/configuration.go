package config

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Configuration struct {
	Telemetry  Telemetry
	LogLevel   string
	PostgreSQL PostgreSQL
	Server     Server
	Positions  Positions
}

type Telemetry struct {
	ServiceName       string
	ServiceVersion    string
	Endpoint          string
	Protocol          string
	Sampler           string
	ElasticEndpoint   string
	ElasticLogin      string
	ElasticPassword   string
	ElasticIndex      string
	ExportTimeout     time.Duration
	MetricInterval    time.Duration
	TraceBatchTimeout time.Duration
	ElasticTimeout    time.Duration
	SamplerRatio      float64
}

type Server struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
}

type PostgreSQL struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

type Positions struct {
	PushURL       string
	CheckInterval time.Duration
	StaleAfter    time.Duration
	PushTimeout   time.Duration
	Workers       int
	QueueSize     int
	BatchSize     int
}

func Load() (*Configuration, error) {
	loader := envLoader{}

	cfg := &Configuration{
		Server: Server{
			Addr:              loader.string("HTTP_ADDR"),
			ReadTimeout:       loader.duration("HTTP_READ_TIMEOUT"),
			ReadHeaderTimeout: loader.duration("HTTP_READ_HEADER_TIMEOUT"),
			WriteTimeout:      loader.duration("HTTP_WRITE_TIMEOUT"),
			IdleTimeout:       loader.duration("HTTP_IDLE_TIMEOUT"),
			ShutdownTimeout:   loader.duration("SHUTDOWN_TIMEOUT"),
			MaxHeaderBytes:    loader.integer("HTTP_MAX_HEADER_BYTES"),
		},
		PostgreSQL: PostgreSQL{
			URL:             loader.string("DATABASE_URL"),
			MaxConns:        loader.int32("DATABASE_MAX_CONNS"),
			MinConns:        loader.int32("DATABASE_MIN_CONNS"),
			MaxConnLifetime: loader.duration("DATABASE_MAX_CONN_LIFETIME"),
			ConnectTimeout:  loader.duration("DATABASE_CONNECT_TIMEOUT"),
			QueryTimeout:    loader.duration("DATABASE_QUERY_TIMEOUT"),
		},
		Positions: Positions{
			PushURL:       loader.string("PUSH_HTTP_URL"),
			CheckInterval: loader.duration("POSITION_CHECK_INTERVAL", "30s"),
			StaleAfter:    loader.duration("POSITION_STALE_AFTER", "60s"),
			PushTimeout:   loader.duration("PUSH_REQUEST_TIMEOUT", "1s"),
			Workers:       loader.integer("POSITION_WORKERS", "4"),
			QueueSize:     loader.integer("POSITION_QUEUE_SIZE", "100"),
			BatchSize:     loader.integer("POSITION_SCAN_BATCH_SIZE", "100"),
		},
		LogLevel: strings.ToLower(loader.string("LOG_LEVEL")),
		Telemetry: Telemetry{
			ServiceName:       envOrDefault("OTEL_SERVICE_NAME", "trip-service"),
			ServiceVersion:    loader.string("SERVICE_VERSION"),
			Endpoint:          strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
			Protocol:          envOrDefault("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf"),
			Sampler:           envOrDefault("OTEL_TRACES_SAMPLER", "parentbased_always_on"),
			ElasticEndpoint:   os.Getenv("ELASTIC_ENDPOINT"),
			ElasticLogin:      os.Getenv("ELASTIC_LOGIN"),
			ElasticPassword:   os.Getenv("ELASTIC_PASSWORD"),
			ElasticIndex:      envOrDefault("ELASTIC_INDEX", "trip-service-logs"),
			ExportTimeout:     loader.milliseconds("OTEL_EXPORTER_OTLP_TIMEOUT"),
			MetricInterval:    loader.milliseconds("OTEL_METRIC_EXPORT_INTERVAL"),
			TraceBatchTimeout: loader.milliseconds("OTEL_BSP_SCHEDULE_DELAY"),
			ElasticTimeout:    loader.duration("ELASTIC_TIMEOUT"),
			SamplerRatio:      loader.ratio("OTEL_TRACES_SAMPLER_ARG"),
		},
	}

	if loader.err != nil {
		return nil, loader.err
	}
	if cfg.Telemetry.Protocol != "http/protobuf" && cfg.Telemetry.Protocol != "grpc" {
		return nil, fmt.Errorf("OTEL_EXPORTER_OTLP_PROTOCOL must be http/protobuf or grpc")
	}
	if err := cfg.Telemetry.validate(); err != nil {
		return nil, err
	}

	for _, setting := range []struct {
		name  string
		value int
	}{
		{name: "POSITION_WORKERS", value: cfg.Positions.Workers},
		{name: "POSITION_QUEUE_SIZE", value: cfg.Positions.QueueSize},
		{name: "POSITION_SCAN_BATCH_SIZE", value: cfg.Positions.BatchSize},
	} {
		if setting.value <= 0 {
			return nil, fmt.Errorf("%s must be positive", setting.name)
		}
	}

	if cfg.Server.MaxHeaderBytes <= 0 {
		return nil, fmt.Errorf("HTTP_MAX_HEADER_BYTES must be positive")
	}
	if cfg.PostgreSQL.MaxConns <= 0 {
		return nil, fmt.Errorf("DATABASE_MAX_CONNS must be positive")
	}
	if cfg.PostgreSQL.MinConns < 0 {
		return nil, fmt.Errorf("DATABASE_MIN_CONNS must not be negative")
	}
	if cfg.PostgreSQL.MinConns > cfg.PostgreSQL.MaxConns {
		return nil, fmt.Errorf("DATABASE_MIN_CONNS must not exceed DATABASE_MAX_CONNS")
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return nil, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn or error")
	}

	return cfg, nil
}

func (c Telemetry) validate() error {
	switch c.Sampler {
	case "always_on", "always_off", "parentbased_always_on", "parentbased_always_off", "traceidratio", "parentbased_traceidratio":
	default:
		return fmt.Errorf("unsupported OTEL_TRACES_SAMPLER")
	}
	for _, endpoint := range []struct {
		name  string
		value string
	}{{name: "OTEL_EXPORTER_OTLP_ENDPOINT", value: c.Endpoint}, {name: "ELASTIC_ENDPOINT", value: c.ElasticEndpoint}} {
		if endpoint.value == "" {
			continue
		}
		parsed, err := url.Parse(endpoint.value)
		if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("%s must be an HTTP URL without credentials, query or fragment", endpoint.name)
		}
	}
	return nil
}

func (l *envLoader) ratio(name string) float64 {
	value := l.string(name)
	if l.err != nil {
		return 0
	}
	ratio, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 1 {
		l.err = fmt.Errorf("%s must be between 0 and 1", name)
		return 0
	}
	return ratio
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

type envLoader struct {
	err error
}

func (l *envLoader) string(name string, fallback ...string) string {
	if l.err != nil {
		return ""
	}

	value, ok := os.LookupEnv(name)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		if len(fallback) != 0 {
			return fallback[0]
		}
		l.err = fmt.Errorf("required environment variable %s is not set", name)
		return ""
	}

	return value
}

func (l *envLoader) duration(name string, fallback ...string) time.Duration {
	value := l.string(name, fallback...)
	if l.err != nil {
		return 0
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		l.err = fmt.Errorf("parse %s: %w", name, err)
		return 0
	}
	if result <= 0 {
		l.err = fmt.Errorf("%s must be positive", name)
		return 0
	}

	return result
}

func (l *envLoader) milliseconds(name string) time.Duration {
	value := l.string(name)
	if l.err != nil {
		return 0
	}
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || milliseconds <= 0 || milliseconds > int64(math.MaxInt64)/int64(time.Millisecond) {
		l.err = fmt.Errorf("%s must be a positive number of milliseconds", name)
		return 0
	}
	return time.Duration(milliseconds) * time.Millisecond
}

func (l *envLoader) int32(name string) int32 {
	value := l.string(name)
	if l.err != nil {
		return 0
	}

	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		l.err = fmt.Errorf("parse %s: %w", name, err)
		return 0
	}

	return int32(result)
}

func (l *envLoader) integer(name string, fallback ...string) int {
	value := l.string(name, fallback...)
	if l.err != nil {
		return 0
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		l.err = fmt.Errorf("parse %s: %w", name, err)
		return 0
	}

	return result
}
