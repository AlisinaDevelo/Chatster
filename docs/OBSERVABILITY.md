# Observability

How Chatster is instrumented today and how you would extend it toward **SLOs** and **distributed tracing**.

## Today

| Signal | Implementation |
|--------|----------------|
| **Logs** | Structured JSON via `log/slog` on stdout (keys suitable for log aggregation). |
| **Metrics** | **Prometheus** exposition at **`GET /metrics`** (process + custom Chatster counters/gauges). |
| **Health** | **`GET /health`** — liveness/readiness-style signal including SQLite ping. |

## Prometheus metrics (custom)

Namespaces and names follow `chatster_*` where applicable. Inspect `/metrics` on a running server for the full set (including Go runtime defaults).

Suggested **recording rules / dashboards** (Grafana) for a real deployment:

- **Traffic:** rate of WS upgrades, rate of messages persisted.
- **Saturation:** connected clients, goroutine count (default `go_*` metrics).
- **Errors:** WS upgrade failures, DB errors (expose via counters when you add labeled error paths).

## SLO sketch (example — not a promise)

These are **illustrative** targets for a small internal deployment; tune with real baselines.

| SLI | Example SLO | Measurement idea |
|-----|-------------|------------------|
| API availability | 99.5% monthly | Synthetic checks on `/health` + edge LB metrics. |
| Message send latency (p99) | < 500 ms internal | Client-side beacon or server histogram once you add one. |
| WS connection success | > 99% of attempts (non-abusive) | Ratio `successful upgrades / attempts` from metrics. |

Chatster does **not** yet ship histogram buckets for message latency; adding `prometheus.Histogram` around `SaveMessage` and broadcast would be the natural next step.

## Tracing (recommended next step)

**OpenTelemetry** (OTel) would add:

- Trace context over HTTP upgrade and first WS frames (where the stack supports it).
- Spans for DB queries and hub broadcast.

Why not bundled here: OTel pulls a larger dependency tree and exporter configuration (OTLP endpoint, sampling). For a portfolio reference, **Prometheus + logs + documented OTel path** keeps the binary smaller while still showing senior-level awareness.

**Adoption outline**

1. Initialize a `TracerProvider` in `main` with resource attributes (`service.name=chatster`).
2. Instrument HTTP mux with `otelhttp` middleware (upgrade routes need careful testing).
3. Use `otel` SQLite wrapper or manual spans in `db` package for query latency.

## Secrets

- No secrets in-repo; inject via environment or a secret manager in production.
- Never enable debug logging of message bodies in shared environments.

See also [OPERATIONS.md](OPERATIONS.md) for probes and log shipping.
