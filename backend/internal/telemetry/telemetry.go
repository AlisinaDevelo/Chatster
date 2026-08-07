// Package telemetry contains Chatster's opt-in tracing boundary.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	serviceName         = "chatster"
	instrumentationName = "github.com/AliSinaDevelo/Chatster"
)

var tracerState struct {
	sync.RWMutex
	tracer trace.Tracer
}

func init() {
	tracerState.tracer = otel.Tracer(instrumentationName)
}

// SetTracerProvider changes the provider used by Chatster's instrumentation.
// It is also useful for tests that install an in-memory exporter.
func SetTracerProvider(provider trace.TracerProvider) {
	if provider == nil {
		provider = noop.NewTracerProvider()
	}

	tracerState.Lock()
	tracerState.tracer = provider.Tracer(instrumentationName)
	tracerState.Unlock()
}

// Start starts a span with the current Chatster tracer. Nil contexts are
// treated as context.Background so instrumentation never interrupts a request.
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}

	tracerState.RLock()
	tracer := tracerState.tracer
	tracerState.RUnlock()
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// MarkError records a bounded error status without copying database errors,
// request data, or message content into the span.
func MarkError(span trace.Span) {
	if span != nil {
		span.SetStatus(codes.Error, "operation failed")
	}
}

// Setup installs the OTLP/HTTP tracer provider only when tracing is enabled.
// The returned shutdown function flushes pending spans and is safe to call
// during process shutdown.
func Setup(ctx context.Context) (shutdown func(context.Context) error, enabled bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	configured, err := tracingConfigured()
	if err != nil {
		return nil, false, err
	}
	if !configured {
		return func(context.Context) error { return nil }, false, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	name := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if name == "" {
		name = serviceName
	}
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(attribute.String("service.name", name)),
	)
	if err != nil {
		return nil, false, fmt.Errorf("create telemetry resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	SetTracerProvider(provider)
	otel.SetTracerProvider(provider)

	return provider.Shutdown, true, nil
}

func tracingConfigured() (bool, error) {
	if raw := strings.TrimSpace(os.Getenv("CHATSTER_OTEL_ENABLED")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return false, fmt.Errorf("CHATSTER_OTEL_ENABLED must be a boolean: %w", err)
		}
		if !enabled {
			return false, nil
		}
	}

	exporter := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_EXPORTER")))
	if exporter == "none" {
		return false, nil
	}
	if exporter != "" && exporter != "otlp" {
		return false, fmt.Errorf("unsupported OTEL_TRACES_EXPORTER %q; use otlp or none", exporter)
	}

	if exporter == "otlp" || strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" || strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")) != "" {
		return true, nil
	}

	return strings.TrimSpace(os.Getenv("CHATSTER_OTEL_ENABLED")) != "", nil
}
