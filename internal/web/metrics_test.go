package web

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRequestLogsCorrelateBoundedHTTPSpans(t *testing.T) {
	var logs bytes.Buffer
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})

	app := &App{metrics: newMetrics(), logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	handler := app.logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/orders/sensitive-order-id", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	handler.ServeHTTP(recorder, request)

	if !strings.Contains(logs.String(), `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`) {
		t.Fatalf("request log missing incoming trace ID: %s", logs.String())
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "GET /api" {
		t.Fatalf("span name = %q, want bounded route name", spans[0].Name)
	}
	if spans[0].SpanContext.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace ID = %s, want incoming trace ID", spans[0].SpanContext.TraceID())
	}
	if strings.Contains(spans[0].Name, "sensitive-order-id") {
		t.Fatalf("span name contains raw URL path: %s", spans[0].Name)
	}
}

func TestRequestLogsUseBoundedRoutes(t *testing.T) {
	var logs bytes.Buffer
	app := &App{metrics: newMetrics(), logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	handler := app.logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/orders/sensitive-order-id", nil))

	if strings.Contains(logs.String(), "sensitive-order-id") {
		t.Fatalf("request log contains raw URL path: %s", logs.String())
	}
	if !strings.Contains(logs.String(), `"path":"/api"`) {
		t.Fatalf("request log missing bounded route: %s", logs.String())
	}
}

func TestPrometheusMetricsUsesBoundedPathsAndStatus(t *testing.T) {
	app := &App{metrics: newMetrics()}
	app.metrics.record(http.MethodGet, metricPath("/api/products/123"), http.StatusOK, 20*time.Millisecond)

	recorder := httptest.NewRecorder()
	app.prometheusMetrics(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `mitte_http_requests_total{method="GET",path="/api",status="200"} 1`) {
		t.Fatalf("metrics missing bounded request series: %s", body)
	}
	if strings.Contains(body, "/api/products/123") {
		t.Fatalf("metrics contain raw URL path: %s", body)
	}
	if !strings.Contains(body, `mitte_http_request_duration_seconds_bucket{method="GET",path="/api",status="200",le="0.05"} 1`) {
		t.Fatalf("metrics missing request duration histogram bucket: %s", body)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("metrics Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
}

func TestMetricPathKeepsCheckoutAndStripeWebhookSeparatelyObservable(t *testing.T) {
	for _, path := range []string{"/api/checkout", "/webhooks/stripe"} {
		if got := metricPath(path); got != path {
			t.Errorf("metricPath(%q) = %q, want %q", path, got, path)
		}
	}
	if got := metricPath("/api/orders/sensitive-order-id"); got != "/api" {
		t.Errorf("metricPath() for dynamic API URL = %q, want /api", got)
	}
}

func TestPrometheusMetricsRequiresBearerTokenWhenConfigured(t *testing.T) {
	app := &App{metrics: newMetrics(), metricsBearerToken: "a-long-random-metrics-token"}
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	app.prometheusMetrics(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status = %d, want 401", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Authorization", "Bearer a-long-random-metrics-token")
	recorder = httptest.NewRecorder()
	app.prometheusMetrics(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authenticated metrics status = %d, want 200", recorder.Code)
	}
}

func TestPrometheusMetricsAcceptsInternalScrapeHost(t *testing.T) {
	handler := New(nil, Options{
		TrustedHosts:       map[string]struct{}{"shop.example.de": {}},
		MetricsBearerToken: "a-long-random-metrics-token",
	})
	request := httptest.NewRequest(http.MethodGet, "http://10.0.0.2:8080/metrics", nil)
	request.Header.Set("Authorization", "Bearer a-long-random-metrics-token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("internal scrape status = %d, want 200", recorder.Code)
	}
}
