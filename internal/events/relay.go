package events

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"mitte/internal/store"
	"mitte/internal/telemetry"
)

// RunRelay drains the transactional outbox. It is run by the dedicated
// order-relay process, independently scalable from the HTTP checkout service.
func RunRelay(ctx context.Context, repository *store.Store, publisher *KafkaPublisher, logger *slog.Logger) {
	RunRelayWithMetrics(ctx, repository, publisher, logger, nil)
}

func RunRelayWithMetrics(ctx context.Context, repository *store.Store, publisher *KafkaPublisher, logger *slog.Logger, metrics *telemetry.WorkerMetrics) {
	for {
		dispatched, err := repository.DispatchNextOutboxEvent(ctx, publisher)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("publish order event", "error", err)
			if metrics != nil {
				metrics.Record("publish", "error")
			}
		} else if dispatched && metrics != nil {
			metrics.Record("publish", "success")
		}
		delay := time.Second
		if dispatched {
			delay = 50 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}
