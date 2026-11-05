package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mitte/internal/config"
	"mitte/internal/grpcserver"
	"mitte/internal/invoiceprofile"
	"mitte/internal/payments"
	"mitte/internal/store"
	"mitte/internal/telemetry"
	"mitte/internal/web"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	// Local development is commonly started with `go run`, which does not read
	// .env files itself. Load it before reading configuration, without ever
	// replacing variables supplied by the shell, container, or deployment.
	if err := config.LoadDevelopmentEnvFile(".env"); err != nil {
		logger.Error("load development environment", "error", err)
		return 1
	}
	config, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	logger, shutdownLogs, err := telemetry.ConfigureLogger(context.Background(), "mitte-web")
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
	shutdownTracing, err := telemetry.ConfigureTracing(context.Background(), "mitte-web")
	if err != nil {
		logger.Warn("configure tracing; continuing without trace export", "error", err)
	}
	defer func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownTracing(shutdownContext); err != nil {
			logger.Error("shutdown tracing", "error", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	repository, err := store.OpenWithPool(ctx, config.DatabaseURL, store.PoolConfig{MaxOpenConns: config.DatabaseMaxOpenConns, MaxIdleConns: config.DatabaseMaxIdleConns})
	if err != nil {
		logger.Error("connect to postgres", "error", err)
		return 1
	}
	defer repository.Close()

	if config.RunMigrations {
		if err := repository.Migrate(ctx); err != nil {
			logger.Error("apply database migrations", "error", err)
			return 1
		}
	}

	grpcListener, err := net.Listen("tcp", config.GRPCListenAddr)
	if err != nil {
		logger.Error("listen for grpc", "address", config.GRPCListenAddr, "error", err)
		return 1
	}
	grpcServer := grpcserver.New(repository, logger)
	defer grpcServer.Stop()
	go func() {
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Error("grpc server stopped unexpectedly", "error", err)
		}
	}()

	var stripeClient payments.CheckoutCreator
	if config.StripeSecretKey != "" {
		stripeClient = payments.NewStripeClient(config.StripeSecretKey)
	}

	server := &http.Server{
		Addr: ":" + config.Port,
		Handler: web.New(repository, web.Options{
			SecureCookies:                   config.SecureCookies,
			TrustedHosts:                    config.TrustedHosts,
			TrustedProxyCIDRs:               config.TrustedProxyCIDRs,
			MetricsBearerToken:              config.MetricsBearerToken,
			RequestTimeout:                  config.RequestTimeout,
			Production:                      config.Environment == "production",
			Logger:                          logger,
			Stripe:                          stripeClient,
			StripeWebhookSecret:             config.StripeWebhookSecret,
			StripeSuccessURL:                config.StripeSuccessURL,
			StripeCancelURL:                 config.StripeCancelURL,
			MittePaySandboxEnabled:     config.MittePaySandboxEnabled,
			MittePayOTPSecret:          config.MittePayOTPSecret,
			MitteTransferEnabled:       config.MitteTransferEnabled,
			MitteTransferIBAN:          config.MitteTransferIBAN,
			MitteTransferRecipient:     config.MitteTransferRecipient,
			MitteInvoiceSandboxEnabled: config.MitteInvoiceSandboxEnabled,
			StaffCredentials:                config.StaffCredentials,
			StaffRoles:                      config.StaffRoles,
			SupplierCredentials:             config.SupplierCredentials,
			InvoiceSeller:                   invoiceprofile.Profile{Name: config.InvoiceSellerName, Address: config.InvoiceSellerAddress, Street: config.InvoiceSellerStreet, PostalCode: config.InvoiceSellerPostalCode, City: config.InvoiceSellerCity, CountryCode: config.InvoiceSellerCountryCode, VATID: config.InvoiceSellerVATID, Email: config.InvoiceSellerEmail, IBAN: config.InvoiceSellerIBAN, PaymentTerms: config.InvoicePaymentTerms, Demo: config.Environment != "production"},
			InvoicePublicBaseURL:            config.PublicBaseURL,
			GeminiAPIKey:                    config.GeminiAPIKey,
			GeminiModel:                     config.GeminiModel,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("server started", "port", config.Port, "grpc_address", config.GRPCListenAddr, "environment", config.Environment)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			return 1
		}
	case signal := <-signals:
		logger.Info("shutdown requested", "signal", signal.String())
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			return 1
		}
		gracefulStopGRPC(shutdownContext, grpcServer)
	}
	return 0
}

type grpcStopper interface {
	GracefulStop()
	Stop()
}

func gracefulStopGRPC(ctx context.Context, server grpcStopper) {
	done := make(chan struct{})
	go func() { server.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		server.Stop()
	}
}
