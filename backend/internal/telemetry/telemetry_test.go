package telemetry

import (
	"context"
	"testing"
)

func TestSetupIsDisabledByDefault(t *testing.T) {
	t.Setenv("CHATSTER_OTEL_ENABLED", "")
	t.Setenv("OTEL_TRACES_EXPORTER", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	shutdown, enabled, err := Setup(context.Background())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if enabled {
		t.Fatal("tracing should be disabled without an opt-in setting")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestSetupRejectsUnsupportedExporter(t *testing.T) {
	t.Setenv("CHATSTER_OTEL_ENABLED", "true")
	t.Setenv("OTEL_TRACES_EXPORTER", "jaeger")

	if _, _, err := Setup(context.Background()); err == nil {
		t.Fatal("setup should reject unsupported exporters")
	}
}
