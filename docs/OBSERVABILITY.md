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

| Metric | Type | What it tells you |
|--------|------|-------------------|
| `chatster_websocket_clients_connected` | Gauge | Current active WebSocket clients. |
| `chatster_websocket_upgrades_total{result}` | Counter | Upgrade attempts by outcome: `ok`, `denied_origin`, `rate_limited`, `upgrade_error`. |
| `chatster_websocket_outbound_drops_total{reason}` | Counter | Outbound drops by reason: `slow_client`, `write_error`. |
| `chatster_chat_messages_ingested_total` | Counter | Valid chat messages accepted for persistence and broadcast. |
| `chatster_chat_messages_rejected_total{reason}` | Counter | Rejected chat inputs by reason: `invalid_username`, `invalid_body`, `rate_limited`. |
| `chatster_chat_message_persist_duration_seconds{result}` | Histogram | SQLite persistence latency for accepted chat/system messages by `ok` / `error`. |
| `chatster_websocket_broadcast_fanout_duration_seconds` | Histogram | Hub fanout enqueue latency for each broadcast message. |

Import [grafana/chatster-dashboard.json](grafana/chatster-dashboard.json) into Grafana with a Prometheus datasource to view:

- **Traffic:** connected clients, message ingest rate, upgrade rate.
- **Latency:** p95 SQLite persist latency and p99 broadcast fanout latency.
- **Errors/abuse:** rejected messages by reason and outbound drops by reason.

Useful PromQL snippets:

```promql
histogram_quantile(
  0.95,
  sum(rate(chatster_chat_message_persist_duration_seconds_bucket[5m])) by (le, result)
)
```

```promql
histogram_quantile(
  0.99,
  sum(rate(chatster_websocket_broadcast_fanout_duration_seconds_bucket[5m])) by (le)
)
```

## SLO sketch (example — not a promise)

These are **illustrative** targets for a small internal deployment; tune with real baselines.

| SLI | Example SLO | Measurement idea |
|-----|-------------|------------------|
| API availability | 99.5% monthly | Synthetic checks on `/health` + edge LB metrics. |
| Message persist latency (p95) | < 50 ms internal | `chatster_chat_message_persist_duration_seconds` histogram. |
| Broadcast fanout latency (p99) | < 100 ms internal | `chatster_websocket_broadcast_fanout_duration_seconds` histogram. |
| WS connection success | > 99% of attempts (non-abusive) | Ratio `successful upgrades / attempts` from metrics. |

Client-perceived message send latency still needs a browser-side beacon or explicit ack path; the current server histograms cover persistence and hub fanout.

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
