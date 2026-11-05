// Command email-relay delivers transactional order mail from the durable
// database outbox. Run it separately from the HTTP service for retry safety.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mitte/internal/config"
	"mitte/internal/notifications"
	"mitte/internal/store"
	"mitte/internal/telemetry"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	_ = config.LoadDevelopmentEnvFile(".env")
	if err := os.Setenv("APP_COMPONENT", "email-relay"); err != nil {
		logger.Error("set app component", "error", err)
		return 1
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	logger, shutdownLogs, err := telemetry.ConfigureLogger(context.Background(), "mitte-email-relay")
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
	productionSMTP := cfg.Environment == "production"
	mailer, err := notifications.NewSMTPMailer(notifications.SMTPConfig{Address: os.Getenv("SMTP_ADDRESS"), Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"), From: os.Getenv("SMTP_FROM"), RequireTLS: productionSMTP})
	if err != nil {
		logger.Error("configure SMTP", "error", err)
		return 1
	}
	var sender store.EmailSender = mailer
	if mirrorAddress := os.Getenv("SMTP_MIRROR_ADDRESS"); mirrorAddress != "" {
		mirror, err := notifications.NewSMTPMailer(notifications.SMTPConfig{Address: mirrorAddress, Username: os.Getenv("SMTP_MIRROR_USERNAME"), Password: os.Getenv("SMTP_MIRROR_PASSWORD"), From: os.Getenv("SMTP_MIRROR_FROM"), RequireTLS: productionSMTP})
		if err != nil {
			logger.Error("configure SMTP mirror", "error", err)
			return 1
		}
		sender = notifications.MirroringMailer{Primary: mailer, Mirror: mirror, OnMirrorError: func(err error) { logger.Error("mirror email", "error", err) }}
	}
	repo, err := store.OpenWithPool(context.Background(), cfg.DatabaseURL, store.PoolConfig{MaxOpenConns: cfg.DatabaseMaxOpenConns, MaxIdleConns: cfg.DatabaseMaxIdleConns})
	if err != nil {
		logger.Error("connect to postgres", "error", err)
		return 1
	}
	defer repo.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	metrics, err := telemetry.StartWorkerMetrics(ctx, "email-relay", cfg.MetricsBearerToken, logger)
	if err != nil {
		logger.Error("start worker metrics", "error", err)
		return 1
	}
	logger.Info("email relay started")
	for {
		didWork, err := repo.DispatchNextEmail(ctx, sender)
		if err != nil {
			logger.Error("dispatch email", "error", err)
			metrics.Record("email_delivery", "error")
		} else if didWork {
			metrics.Record("email_delivery", "success")
		}
		if !didWork {
			select {
			case <-ctx.Done():
				return 0
			case <-time.After(time.Second):
			}
		}
		if ctx.Err() != nil {
			return 0
		}
	}
}
