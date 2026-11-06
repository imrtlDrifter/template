package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/imrtlDrifter/template/internal/config"
	"github.com/imrtlDrifter/template/internal/handlers"
	"github.com/imrtlDrifter/template/internal/handlers/middlewares"
	handlerv1 "github.com/imrtlDrifter/template/internal/handlers/v1"
	createhandler "github.com/imrtlDrifter/template/internal/handlers/v1/trip/create"
	finishhandler "github.com/imrtlDrifter/template/internal/handlers/v1/trip/finish"
	gethandler "github.com/imrtlDrifter/template/internal/handlers/v1/trip/get"
	"github.com/imrtlDrifter/template/internal/infra/postgres"
	"github.com/imrtlDrifter/template/internal/infra/postgres/transaction"
	"github.com/imrtlDrifter/template/internal/infra/server"
	"github.com/imrtlDrifter/template/internal/infra/telemetry"
	"github.com/imrtlDrifter/template/internal/infra/telemetry/logger"
	"github.com/imrtlDrifter/template/internal/infra/telemetry/tracing"
	postgresrepo "github.com/imrtlDrifter/template/internal/repositories/postgres"
	"github.com/imrtlDrifter/template/internal/service"
	createservice "github.com/imrtlDrifter/template/internal/service/trip/create"
	finishservice "github.com/imrtlDrifter/template/internal/service/trip/finish"
	getservice "github.com/imrtlDrifter/template/internal/service/trip/get"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricSDK "go.opentelemetry.io/otel/sdk/metric"
	traceSDK "go.opentelemetry.io/otel/sdk/trace"
)

type Container struct {
	db              *postgres.Database
	server          *server.Server
	logger          *logger.Logger
	metrics         *metricSDK.MeterProvider
	tracing         *traceSDK.TracerProvider
	activeTrips     metric.Registration
	shutdownTimeout time.Duration
}

func New(ctx context.Context, cfg *config.Configuration) (*Container, error) {
	elastic, err := logger.NewElastic(ctx, cfg.Telemetry)
	if err != nil {
		return nil, fmt.Errorf("init elastic logger: %w", err)
	}
	var level slog.Level
	if levelErr := level.UnmarshalText([]byte(cfg.LogLevel)); levelErr != nil {
		return nil, fmt.Errorf("parse log level: %w", levelErr)
	}
	log := logger.NewLogger(os.Stdout, logger.Config{Service: cfg.Telemetry.ServiceName, Level: level}, elastic)
	app := &Container{logger: log, shutdownTimeout: cfg.Server.ShutdownTimeout}
	initialized := false
	defer func() {
		if !initialized {
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Server.ShutdownTimeout)
			defer cancel()
			if closeErr := app.Close(shutdownCtx); closeErr != nil {
				log.ErrorContext(shutdownCtx, closeErr, "initialization cleanup failed")
			}
		}
	}()
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.WarnContext(context.WithoutCancel(ctx), err, "telemetry export failed")
	}))
	app.tracing, err = tracing.InitTracing(ctx, cfg.Telemetry)
	if err != nil {
		return nil, fmt.Errorf("init tracing: %w", err)
	}
	app.metrics, err = telemetry.InitMetrics(ctx, cfg.Telemetry)
	if err != nil {
		return nil, fmt.Errorf("init metrics: %w", err)
	}
	metrics, err := telemetry.NewMetrics(app.metrics, log)
	if err != nil {
		return nil, fmt.Errorf("create metrics: %w", err)
	}
	poolConfig, err := postgres.NewConfig(cfg.PostgreSQL)
	if err != nil {
		return nil, fmt.Errorf("init postgres config: %w", err)
	}
	poolConfig.ConnConfig.Tracer = telemetry.NewPostgresTracer(app.tracing, metrics.DBDuration)
	connectCtx, cancel := context.WithTimeout(ctx, cfg.PostgreSQL.ConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	app.db = postgres.NewDatabase(pool, cfg.PostgreSQL.QueryTimeout)
	if pingErr := pool.Ping(connectCtx); pingErr != nil {
		return nil, fmt.Errorf("ping postgres: %w", pingErr)
	}
	txManager := transaction.NewTransactionManager(pool, cfg.PostgreSQL.QueryTimeout)
	tripRepo := postgresrepo.NewTripRepository(pool, cfg.PostgreSQL.QueryTimeout)
	statusHistoryRepo := postgresrepo.NewStatusHistoryRepository(pool, cfg.PostgreSQL.QueryTimeout)
	idempotencyRepo := postgresrepo.NewIdempotencyRepository(pool, cfg.PostgreSQL.QueryTimeout)
	repositoryErrors := postgresrepo.NewRepositoryErrors()
	app.activeTrips, err = metrics.ObserveActiveTrips(tripRepo)
	if err != nil {
		return nil, fmt.Errorf("register active trips: %w", err)
	}
	tripCreate := createservice.NewTripService(tripRepo, statusHistoryRepo, idempotencyRepo, txManager, repositoryErrors, metrics)
	tripGet := getservice.NewTripService(tripRepo, repositoryErrors)
	tripFinish := finishservice.NewTripService(tripRepo, statusHistoryRepo, txManager, repositoryErrors, metrics)
	services, err := service.NewService(tripCreate, tripGet, tripFinish)
	if err != nil {
		return nil, fmt.Errorf("init service: %w", err)
	}
	createHandler := createhandler.NewHandler(services.TripCreate)
	getHandler := gethandler.NewHandler(services.TripGet)
	finishHandler := finishhandler.NewHandler(services.TripFinish)
	handlerV1 := handlerv1.NewHandler(app.db, createHandler, getHandler, finishHandler)
	httpHandler := handlers.NewHandler(handlerV1, middlewares.NewRecovery(log).Middleware, middlewares.NewTracing(app.tracing.Tracer(cfg.Telemetry.ServiceName), tracing.NewPropagator()).Middleware, log.GinMiddleware, (middlewares.Metrics{RequestDuration: metrics.RequestDuration, ActiveRequests: metrics.ActiveRequests}).Middleware, logger.WithRequestID())
	app.server = server.NewServer(cfg, httpHandler.Init())
	initialized = true
	return app, nil
}

func (c *Container) Run(ctx context.Context) error {
	results := make(chan error, 1)
	go func() { results <- c.server.Run() }()
	c.logger.InfoContext(ctx, "service started")
	var runErr error
	stopped := false
	select {
	case runErr = <-results:
		stopped = true
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdownTimeout)
	defer cancel()
	c.logger.InfoContext(shutdownCtx, "service stopping")
	stopErr := c.server.Stop(shutdownCtx)
	if stopErr != nil {
		stopErr = errors.Join(stopErr, c.server.Close())
	}
	if !stopped {
		select {
		case runErr = <-results:
		case <-shutdownCtx.Done():
			runErr = shutdownCtx.Err()
		}
	}
	if errors.Is(runErr, http.ErrServerClosed) {
		runErr = nil
	}
	closeErr := c.Close(shutdownCtx)
	if runErr != nil || stopErr != nil || closeErr != nil {
		return fmt.Errorf("stop service: %w", errors.Join(runErr, stopErr, closeErr))
	}
	c.logger.InfoContext(shutdownCtx, "service stopped")
	return nil
}

func (c *Container) Close(ctx context.Context) error {
	var closeErr error
	if c.metrics != nil {
		closeErr = errors.Join(closeErr, c.metrics.Shutdown(ctx))
	}
	if c.activeTrips != nil {
		closeErr = errors.Join(closeErr, c.activeTrips.Unregister())
	}
	if c.tracing != nil {
		closeErr = errors.Join(closeErr, c.tracing.Shutdown(ctx))
	}
	if c.db != nil {
		c.db.Close()
	}
	return errors.Join(closeErr, c.logger.Shutdown(ctx))
}
