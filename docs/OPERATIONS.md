# Operations

Runbook-style notes for operating Chatster beyond local development.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CHATSTER_HTTP_ADDR` | `:8080` | HTTP listen address |
| `CHATSTER_DB_PATH` | `./chatster.db` | SQLite database file path |
| `CHATSTER_ALLOWED_ORIGINS` | _(empty)_ | Comma-separated WebSocket `Origin` values; empty allows all origins |
| `CHATSTER_WS_UPGRADE_RPS` | `5` | Per-IP WebSocket upgrades per second (`0` disables) |
| `CHATSTER_WS_UPGRADE_BURST` | `10` | Burst size for the upgrade limiter |
| `CHATSTER_MESSAGE_RPS` | `5` | Per-client chat messages per second (`0` disables) |
| `CHATSTER_MESSAGE_BURST` | `10` | Burst size for the message limiter |

## Health checks

`GET /health` returns JSON:

- **200** — `status` is `ok`, `database` is `ok`.
- **503** — `status` is `degraded` when the database ping fails within ~2s.

Example:

```json
{"status":"ok","database":"ok","service":"chatster"}
```

Use this endpoint for Kubernetes liveness/readiness or load balancer probes.

## Metrics

`GET /metrics` exposes **Prometheus** text format (Go runtime collectors plus `chatster_*` counters/gauges). Scrape from your Prometheus server or agent; see [OBSERVABILITY.md](OBSERVABILITY.md) for naming and SLO guidance.

## Logging

The API emits **structured JSON logs** on stdout via `log/slog` (Go 1.22+). Aggregate with your log stack (CloudWatch, Loki, Datadog, etc.).

## Process lifecycle

The server handles **SIGINT** and **SIGTERM** and attempts **graceful HTTP shutdown** with a **30s** timeout. Long-lived WebSocket connections may delay full drain; scale this pattern if you add a hub shutdown hook.

## Docker

From the repository root:

```bash
docker compose up --build
```

- API: [http://localhost:8080](http://localhost:8080) (WebSocket: `ws://localhost:8080/ws`)
- Web UI: [http://localhost:3000](http://localhost:3000) (static build; browser connects to `ws://localhost:8080/ws`)

SQLite data persists in the **`chatster-data`** Docker volume.

## Persistence

Startup applies ordered SQLite migrations and records them in `schema_migrations`. Check that ledger before serving real traffic after a schema change.

## Backups

Copy the SQLite file while the process is stopped, or use SQLite’s backup API / `.backup` for online copies if you extend the service.

## Production hardening (checklist)

- Terminate TLS at the edge; use **`wss://`** for WebSockets.
- Set **`CHATSTER_ALLOWED_ORIGINS`** to match your static app origins (see [THREAT_MODEL.md](THREAT_MODEL.md)).
- Run the API as a non-root user (Dockerfile already uses a dedicated user).
- Monitor `/health`, **`/metrics`**, log volume, and DB disk growth.
