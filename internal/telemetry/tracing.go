package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func ConfigureTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")) == "" &&
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) == "" {
		return func(context.Context) error { return nil }, nil
	}

	sampleRatio := 0.1
	if raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLE_RATIO")); raw != "" {
		ratio, err := strconv.ParseFloat(raw, 64)
		if err != nil || ratio < 0 || ratio > 1 {
			return func(context.Context) error { return nil }, fmt.Errorf("OTEL_TRACES_SAMPLE_RATIO must be a number from 0 to 1")
		}
		sampleRatio = ratio
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return func(context.Context) error { return nil }, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	resource, err := sdkresource.New(ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return func(context.Context) error { return nil }, fmt.Errorf("create telemetry resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return provider.Shutdown, nil
}
