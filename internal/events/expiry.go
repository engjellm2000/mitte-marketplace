package events

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"mitte/internal/telemetry"
)

// PendingOrderExpirer is kept deliberately small so payment expiry remains an
// independently testable database concern rather than a Kafka responsibility.
type PendingOrderExpirer interface {
	ExpirePendingOrders(context.Context, int) (int, error)
}

// RunPaymentExpiryWorker releases reservations for unpaid, expired checkouts.
// It has no Kafka dependency and runs even while event delivery is unavailable.
func RunPaymentExpiryWorker(ctx context.Context, repository PendingOrderExpirer, logger *slog.Logger, interval time.Duration, batchSize int) {
	RunPaymentExpiryWorkerWithMetrics(ctx, repository, logger, interval, batchSize, nil)
}

func RunPaymentExpiryWorkerWithMetrics(ctx context.Context, repository PendingOrderExpirer, logger *slog.Logger, interval time.Duration, batchSize int, metrics *telemetry.WorkerMetrics) {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = time.Minute
	}
	if batchSize < 1 {
		batchSize = 100
	}
	run := func() {
		expired, err := repository.ExpirePendingOrders(ctx, batchSize)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("expire pending orders", "error", err)
			if metrics != nil {
				metrics.Record("payment_expiry", "error")
			}
			return
		}
		if err == nil && metrics != nil {
			metrics.Record("payment_expiry", "success")
		}
		if expired > 0 {
			logger.Info("expired pending orders", "count", expired)
		}
	}

	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
