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
	config, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	logger, shutdownLogs, err := telemetry.ConfigureLogger(context.Background(), "mitte-order-relay")
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
	if !config.OrderRelayEnabled || len(config.KafkaBrokers) == 0 {
		logger.Error("order relay requires ORDER_RELAY_ENABLED=true and KAFKA_BROKERS")
		return 1
	}
	startupContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repository, err := store.OpenWithPool(startupContext, config.DatabaseURL, store.PoolConfig{MaxOpenConns: config.DatabaseMaxOpenConns, MaxIdleConns: config.DatabaseMaxIdleConns})
	if err != nil {
		logger.Error("connect to postgres", "error", err)
		return 1
	}
	defer repository.Close()
	publisher, err := events.NewKafkaPublisher(config.KafkaBrokers, config.KafkaTopic, events.KafkaSecurityConfig{
		TLSEnabled:    config.KafkaTLSEnabled,
		TLSCACert:     config.KafkaTLSCACert,
		SASLMechanism: config.KafkaSASLMechanism,
		SASLUsername:  config.KafkaSASLUsername,
		SASLPassword:  config.KafkaSASLPassword,
	})
	if err != nil {
		logger.Error("configure Kafka publisher", "error", err)
		return 1
	}
	defer publisher.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	metrics, err := telemetry.StartWorkerMetrics(ctx, "order-relay", config.MetricsBearerToken, logger)
	if err != nil {
		logger.Error("start worker metrics", "error", err)
		return 1
	}
	logger.Info("order relay started", "topic", config.KafkaTopic)
	events.RunRelayWithMetrics(ctx, repository, publisher, logger, metrics)
	return 0
}
