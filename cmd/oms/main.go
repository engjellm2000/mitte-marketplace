package main

import (
	"context"
	"log/slog"
	"mitte/internal/config"
	"mitte/internal/invoiceprofile"
	"mitte/internal/oms"
	"mitte/internal/payments"
	"mitte/internal/store"
	"mitte/internal/telemetry"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	os.Exit(run())
}

func run() int {
	_ = config.LoadDevelopmentEnvFile(".env")
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		return 1
	}
	logger, shutdownLogs, err := telemetry.ConfigureLogger(context.Background(), "mitte-oms")
	if err != nil {
		logger.Warn("configure log export; continuing with local JSON logs", "error", err)
	}
	slog.SetDefault(logger)
	defer func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownLogs(shutdownContext); err != nil {
			logger.Error("shutdown log export", "error", err)
		}
	}()
	shutdownTracing, err := telemetry.ConfigureTracing(context.Background(), "mitte-oms")
	if err != nil {
		logger.Warn("configure tracing; continuing without trace export", "error", err)
	}
	defer func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownTracing(shutdownContext); err != nil {
			slog.Error("shutdown tracing", "error", err)
		}
	}()
	key := strings.TrimSpace(os.Getenv("OMS_API_KEY"))
	if key == "" {
		slog.Error("OMS_API_KEY is required")
		return 1
	}
	username, password := strings.TrimSpace(os.Getenv("OMS_USERNAME")), os.Getenv("OMS_PASSWORD")
	if username == "" || password == "" {
		slog.Error("OMS_USERNAME and OMS_PASSWORD are required")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := store.OpenWithPool(ctx, cfg.DatabaseURL, store.PoolConfig{MaxOpenConns: cfg.DatabaseMaxOpenConns, MaxIdleConns: cfg.DatabaseMaxIdleConns})
	if err != nil {
		slog.Error("connect database", "error", err)
		return 1
	}
	defer repo.Close()
	if cfg.RunMigrations {
		if err := repo.Migrate(ctx); err != nil {
			slog.Error("migrate", "error", err)
			return 1
		}
	}
	port := os.Getenv("OMS_PORT")
	if port == "" {
		port = "8081"
	}
	var refunder payments.Refunder
	if cfg.StripeSecretKey != "" {
		refunder = payments.NewStripeClient(cfg.StripeSecretKey)
	}
	server := &http.Server{Addr: ":" + port, Handler: oms.New(repo, oms.Options{APIKey: key, MetricsBearerToken: cfg.MetricsBearerToken, Username: username, Password: password, Refunder: refunder, PublicBaseURL: cfg.PublicBaseURL, Seller: invoiceprofile.Profile{Name: cfg.InvoiceSellerName, Address: cfg.InvoiceSellerAddress, Street: cfg.InvoiceSellerStreet, PostalCode: cfg.InvoiceSellerPostalCode, City: cfg.InvoiceSellerCity, CountryCode: cfg.InvoiceSellerCountryCode, VATID: cfg.InvoiceSellerVATID, Email: cfg.InvoiceSellerEmail, IBAN: cfg.InvoiceSellerIBAN, PaymentTerms: cfg.InvoicePaymentTerms, Demo: cfg.Environment != "production"}, RequestTimeout: cfg.RequestTimeout, SecureCookies: cfg.SecureCookies, Logger: slog.Default()}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	slog.Info("oms started", "port", port)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("oms stopped unexpectedly", "error", err)
			return 1
		}
	case <-signals:
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			slog.Error("oms graceful shutdown failed", "error", err)
			return 1
		}
		if err := <-serverErrors; err != nil && err != http.ErrServerClosed {
			slog.Error("oms stopped unexpectedly", "error", err)
			return 1
		}
	}
	return 0
}
