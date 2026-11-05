package telemetry

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type logRecordHandler struct {
	record slog.Record
}

func (h *logRecordHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *logRecordHandler) Handle(_ context.Context, record slog.Record) error {
	h.record = record.Clone()
	return nil
}

func (h *logRecordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *logRecordHandler) WithGroup(string) slog.Handler { return h }

func TestRedactingHandlerRemovesSensitiveFieldsRecursively(t *testing.T) {
	capture := &logRecordHandler{}
	handler := redactingHandler{next: capture}
	record := slog.NewRecord(time.Now(), slog.LevelError, "operation failed", 0)
	record.AddAttrs(
		slog.String("order_id", "ord_sensitive"),
		slog.String("path", "/api/orders/ord_sensitive"),
		slog.String("error", "database error for ord_sensitive"),
		slog.Group("context",
			slog.String("customer_email", "buyer@example.test"),
			slog.String("request_id", "req-safe"),
		),
	)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	var keys []string
	capture.record.Attrs(func(attr slog.Attr) bool {
		keys = append(keys, attr.Key)
		if attr.Key == "context" {
			for _, nested := range attr.Value.Group() {
				keys = append(keys, "context."+nested.Key)
			}
		}
		return true
	})
	want := []string{"context", "context.request_id"}
	if len(keys) != len(want) {
		t.Fatalf("retained attribute keys = %v, want %v", keys, want)
	}
	for index := range want {
		if keys[index] != want[index] {
			t.Fatalf("retained attribute keys = %v, want %v", keys, want)
		}
	}
}

func TestConfigureLoggerFallsBackToJSONWithoutOTLPEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")
	logger, shutdown, err := ConfigureLogger(context.Background(), "mitte-test")
	if err != nil {
		t.Fatalf("ConfigureLogger() error = %v", err)
	}
	if logger == nil || shutdown == nil {
		t.Fatal("ConfigureLogger() returned a nil logger or shutdown function")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}
