package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/imrtlDrifter/template/internal/config"
	"github.com/imrtlDrifter/template/internal/container"
)

func main() {
	if err := run(); err != nil {
		log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "trip-service", "trace_id", "", "span_id", "")
		log.Error("application stopped", "err", err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	app, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init container: %w", err)
	}

	return app.Run(ctx)
}
