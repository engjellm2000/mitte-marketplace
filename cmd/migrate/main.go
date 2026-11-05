package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"mitte/internal/config"
	"mitte/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := config.LoadDevelopmentEnvFile(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Component != "migrate" {
		return errInvalidComponent
	}
	if !cfg.RunMigrations {
		return errMigrationsDisabled
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	repository, err := store.OpenWithPool(ctx, cfg.DatabaseURL, store.PoolConfig{
		MaxOpenConns: cfg.DatabaseMaxOpenConns,
		MaxIdleConns: cfg.DatabaseMaxIdleConns,
	})
	if err != nil {
		return err
	}
	defer repository.Close()

	if err := repository.Migrate(ctx); err != nil {
		return err
	}
	logger.Info("database migrations completed")
	return nil
}

var (
	errInvalidComponent   = migrationError("APP_COMPONENT must be migrate")
	errMigrationsDisabled = migrationError("RUN_MIGRATIONS must be true")
)

type migrationError string

func (e migrationError) Error() string { return string(e) }
