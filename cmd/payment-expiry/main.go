// Command payment-expiry releases inventory reservations for checkouts that
// were never paid. It is intentionally independent of Kafka and the relay.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mitte/internal/config"
	"mitte/internal/events"
	"mitte/internal/store"
	"mitte/internal/telemetry"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	logger, shutdownLogs, err := telemetry.ConfigureLogger(context.Background(), "mitte-payment-expiry")
	if err != nil {
		logger.Warn("configure log export; continuing with local JSON logs", "error", err)
	}
	defer func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownLogs(shutdownContext); err != nil {
			logger.Error("shutdown log export", "error", err)
		}
	}()
	startupContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repository, err := store.OpenWithPool(startupContext, cfg.DatabaseURL, store.PoolConfig{MaxOpenConns: cfg.DatabaseMaxOpenConns, MaxIdleConns: cfg.DatabaseMaxIdleConns})
	if err != nil {
		logger.Error("connect to postgres", "error", err)
		return 1
	}
	defer repository.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	metrics, err := telemetry.StartWorkerMetrics(ctx, "payment-expiry", cfg.MetricsBearerToken, logger)
	if err != nil {
		logger.Error("start worker metrics", "error", err)
		return 1
	}
	logger.Info("payment expiry worker started")
	events.RunPaymentExpiryWorkerWithMetrics(ctx, repository, logger, time.Minute, 100, metrics)
	return 0
}
