# Observability

How Chatster is instrumented today and how you would extend it toward **SLOs** and **distributed tracing**.

## Today

| Signal | Implementation |
|--------|----------------|
| **Logs** | Structured JSON via `log/slog` on stdout (keys suitable for log aggregation). |
| **Metrics** | **Prometheus** exposition at **`GET /metrics`** (process + custom Chatster counters/gauges). |
| **Health** | **`GET /health`** — liveness/readiness-style signal including the selected repository ping. |

## Prometheus metrics (custom)

Namespaces and names follow `chatster_*` where applicable. Inspect `/metrics` on a running server for the full set (including Go runtime defaults).

| Metric | Type | What it tells you |
|--------|------|-------------------|
| `chatster_websocket_clients_connected` | Gauge | Current active WebSocket clients. |
| `chatster_websocket_upgrades_total{result}` | Counter | Upgrade attempts by outcome: `ok`, `denied_origin`, `rate_limited`, `upgrade_error`, `draining`. |
| `chatster_websocket_drains_started_total` | Counter | WebSocket hub drain operations started during process shutdown. |
| `chatster_websocket_drain_duration_seconds` | Histogram | Duration of each WebSocket hub drain operation. |
| `chatster_websocket_drain_clients_remaining` | Gauge | Client loops that have not reported completion during a drain. |
| `chatster_websocket_drain_forced_closes_total` | Counter | Clients still active when the shutdown drain deadline expired. |
| `chatster_websocket_outbound_drops_total{reason}` | Counter | Outbound drops by reason: `slow_client`, `write_error`. |
| `chatster_chat_messages_ingested_total` | Counter | Valid chat messages accepted for persistence and broadcast. |
| `chatster_chat_messages_rejected_total{reason}` | Counter | Rejected chat inputs by reason: `invalid_username`, `invalid_body`, `rate_limited`. |
| `chatster_chat_messages_pruned_total` | Counter | Persisted chat messages removed by the startup retention policy. |
| `chatster_moderation_audit_events_pruned_total` | Counter | Moderation audit events removed by the startup retention policy. |
| `chatster_chat_message_persist_duration_seconds{result}` | Histogram | Persistence latency for accepted chat/system messages by `ok` / `error`, across the selected repository. |
| `chatster_websocket_broadcast_fanout_duration_seconds` | Histogram | Hub fanout enqueue latency for each broadcast message. |

Import [grafana/chatster-dashboard.json](grafana/chatster-dashboard.json) into Grafana with a Prometheus datasource to view:

- **Traffic:** connected clients, message ingest rate, upgrade rate.
- **Latency:** p95 repository persist latency and p99 broadcast fanout latency.
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

## Tracing (opt-in)

Chatster includes a small OpenTelemetry tracing path, but it is disabled unless
configured. The default local and CI process creates no exporter, makes no
collector connection, and keeps the existing Prometheus and `slog` signals
unchanged.

The application uses manual spans at boundaries that cross goroutines or storage
adapters:

| Span | Important attributes | Parent |
|------|----------------------|--------|
| `chatster.http.request` | HTTP method and matched route template | incoming request context |
| `chatster.storage.ping` | operation | health request |
| `chatster.storage.history` | normalized room and bounded limit | history request or WebSocket session |
| `chatster.websocket.session` | normalized room and lifecycle result | HTTP upgrade request |
| `chatster.storage.save_message` | normalized room and message type | WebSocket session where available |
| `chatster.storage.moderation_event` | bounded rejection reason | WebSocket session where available |
| `chatster.websocket.broadcast.enqueue` | normalized room, message type, queue result | message or notification operation |

Message bodies, usernames, session IDs, URLs with query values, secrets, and
database error strings are intentionally excluded from span attributes and
statuses. Room names are normalized by the existing database boundary before
they are used as trace attributes.

### Collector setup

Tracing uses the OTLP/HTTP exporter and the standard OpenTelemetry environment
variables. Enable it explicitly when a collector is available:

```bash
export CHATSTER_OTEL_ENABLED=true
export OTEL_SERVICE_NAME=chatster
export OTEL_TRACES_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
go run .
```

Use `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` when the collector requires a complete
trace endpoint, or add the standard OTLP headers and timeout variables for the
deployment. `OTEL_TRACES_EXPORTER=none` disables tracing. An OTLP exporter or
OTLP endpoint also opts in without the Chatster flag; the explicit flag is the
recommended production setting because it makes intent clear.

The provider uses `OTEL_SERVICE_NAME` when present and otherwise sets
`service.name=chatster`. The SDK also reads the standard sampler variables, for
example:

```bash
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.10
```

On shutdown, `main` stops the HTTP server and WebSocket hub, then gives the
provider up to ten seconds to flush pending spans. Collector outages do not
change message handling; they are reported through the exporter shutdown path.

Traces answer which request or connection was slow; Prometheus histograms
answer how often and how long across the service; structured logs provide the
bounded operational event and error context. Keep all three signals free of
message content and credentials when adding new instrumentation.

## Secrets

- No secrets in-repo; inject via environment or a secret manager in production.
- Never enable debug logging of message bodies in shared environments.

See also [OPERATIONS.md](OPERATIONS.md) for probes and log shipping.
