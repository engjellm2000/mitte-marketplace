package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	otlploggrpc "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

func ConfigureLogger(ctx context.Context, serviceName string) (*slog.Logger, func(context.Context) error, error) {
	jsonHandler := slog.NewJSONHandler(os.Stderr, nil)
	if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT")) == "" &&
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) == "" {
		return slog.New(jsonHandler), func(context.Context) error { return nil }, nil
	}

	exporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return slog.New(jsonHandler), func(context.Context) error { return nil }, err
	}
	resource, err := sdkresource.New(ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return slog.New(jsonHandler), func(context.Context) error { return nil }, err
	}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(resource),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)
	otelLogger := redactingHandler{next: otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(provider))}
	return slog.New(multiHandler{jsonHandler, otelLogger}), provider.Shutdown, nil
}

type redactingHandler struct {
	next slog.Handler
}

func (h redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	filtered := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		if safe, keep := safeTelemetryAttr(attr); keep {
			filtered.AddAttrs(safe)
		}
		return true
	})
	return h.next.Handle(ctx, filtered)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	safe := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		if filtered, keep := safeTelemetryAttr(attr); keep {
			safe = append(safe, filtered)
		}
	}
	return redactingHandler{next: h.next.WithAttrs(safe)}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{next: h.next.WithGroup(name)}
}

func safeTelemetryAttr(attr slog.Attr) (slog.Attr, bool) {
	key := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(attr.Key))
	switch key {
	case "address", "author", "card", "clientip", "customeremail", "customername",
		"email", "emailaddress", "error", "ip", "ipaddress", "lasterror", "order",
		"orderid", "customerid", "panic", "password", "paymentintentid", "phone",
		"postalcode", "query", "recipient", "requestpath", "requesturl", "sessionid",
		"stack", "street", "stripecheckoutsessionid", "token", "url", "userid",
		"username", "zip":
		return slog.Attr{}, false
	}
	if key == "path" && !safeRoute(attr.Value.String()) {
		return slog.Attr{}, false
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		filtered := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			if safe, keep := safeTelemetryAttr(child); keep {
				filtered = append(filtered, safe)
			}
		}
		attr.Value = slog.GroupValue(filtered...)
	}
	return attr, true
}

func safeRoute(path string) bool {
	switch path {
	case "/", "/api", "/api/checkout", "/auth", "/healthz", "/metrics", "/other",
		"/page", "/readyz", "/static", "/v1/fulfillments", "/v1/orders",
		"/webhooks", "/webhooks/stripe":
		return true
	default:
		return false
	}
}

type multiHandler []slog.Handler

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, handler := range h {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make(multiHandler, len(h))
	for i, handler := range h {
		next[i] = handler.WithAttrs(attrs)
	}
	return next
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	next := make(multiHandler, len(h))
	for i, handler := range h {
		next[i] = handler.WithGroup(name)
	}
	return next
}
