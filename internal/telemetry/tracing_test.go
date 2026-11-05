package telemetry

import (
	"context"
	"testing"
)

func TestConfigureTracingIsDisabledWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_SAMPLE_RATIO", "not-a-ratio")

	shutdown, err := ConfigureTracing(context.Background(), "mitte-test")
	if err != nil {
		t.Fatalf("ConfigureTracing() error = %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestConfigureTracingRejectsInvalidSampleRatio(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_SAMPLE_RATIO", "1.1")

	shutdown, err := ConfigureTracing(context.Background(), "mitte-test")
	if err == nil {
		t.Fatal("ConfigureTracing() accepted a sample ratio greater than 1")
	}
	if shutdown == nil {
		t.Fatal("ConfigureTracing() returned a nil shutdown function on error")
	}
}
