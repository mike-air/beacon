// Chapter 36 — distributed tracing. Three nouns and that is the whole model:
//
//	span    one unit of work, with a start, an end, and attributes
//	trace   a tree of spans that share a trace ID
//	context the context.Context that carries the currently-active span
//
// The payoff teams actually adopt OTel for is not the flame graph. It is the
// trace_id on every log line, so a slow request in a dashboard becomes the
// exact log lines for that request in one click. The flame graph is a bonus.
//
// [verbatim ch36] with a nil-safe Shutdown so main can defer it unconditionally.

package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig is what SetupTracing needs. [glue: the chapter passes its whole
// config.Config; this package must not import internal/config, so it takes the
// four fields it uses.]
type TracingConfig struct {
	Endpoint    string // e.g. "localhost:4317"; empty disables tracing
	ServiceName string
	Version     string
	Env         string
	SampleRatio float64
}

// SetupTracing initialises the global OTel tracer and returns a shutdown
// function. Call it once from main; defer the shutdown.
func SetupTracing(ctx context.Context, cfg TracingConfig) (func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		// Tracing off. Return a no-op so main's defer needs no special case.
		return func(context.Context) error { return nil }, nil
	}

	// OTLP gRPC exporter — speaks to a local collector by default.
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(), // local; TLS in prod
	)
	if err != nil {
		return nil, fmt.Errorf("otel exporter: %w", err)
	}

	// Resource attributes — these tag every span with service info.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			semconv.DeploymentEnvironment(cfg.Env),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	// The provider batches spans and ships them to the exporter.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		// ParentBased: if an upstream service already decided to sample this
		// trace, honour that decision, or you get half a trace — the worst of
		// both worlds.
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRatio),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// Tracer is the one accessor the rest of the codebase uses, so no package has
// to remember the instrumentation name.
func Tracer(name string) trace.Tracer { return otel.Tracer(name) }
