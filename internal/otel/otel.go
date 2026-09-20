package otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"backend/internal/config"
)

// Init sets up OpenTelemetry tracing and returns a shutdown function.
// When tracing is disabled, it returns a no-op shutdown and no error.
func Init(ctx context.Context, cfg *config.ObservabilityConfig, serviceName string) (func(context.Context) error, error) {
	if !cfg.Tracing.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	// Fail fast on misconfiguration. The OTLP gRPC client dials lazily,
	// so without this check an empty endpoint would silently produce
	// a tracer that drops every span. Mirrors ObservabilityConfig.Validate.
	if cfg.Tracing.Endpoint == "" {
		return nil, fmt.Errorf("tracing enabled but no endpoint configured")
	}

	// Set up the OTLP gRPC exporter
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Tracing.Endpoint),
	}
	if cfg.Tracing.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Build resource with service identity
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	// Configure the tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.Tracing.SampleRate)),
	)

	// Set global propagators (W3C Trace Context + Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}

	return shutdown, nil
}
