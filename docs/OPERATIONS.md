# Operations

Runbook-style notes for operating Chatster beyond local development.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CHATSTER_HTTP_ADDR` | `PORT` or `:8080` | HTTP listen address; numeric `PORT` is used when this is unset |
| `CHATSTER_STORAGE` | `sqlite` | Storage backend; `postgres` opts into the pooled Postgres adapter and unknown values fail startup |
| `CHATSTER_DB_PATH` | `./chatster.db` | SQLite database file path |
| `CHATSTER_POSTGRES_DSN` | _(required in Postgres mode)_ | Secret Postgres connection string; never logged |
| `CHATSTER_POSTGRES_MIN_CONNS` | `2` | Postgres pool minimum; positive values only |
| `CHATSTER_POSTGRES_MAX_CONNS` | `10` | Postgres pool maximum; must be at least the minimum |
| `CHATSTER_STATIC_DIR` | _(empty)_ | Optional built frontend directory served by the Go backend |
| `CHATSTER_ALLOWED_ORIGINS` | _(empty)_ | HTTP/WebSocket `Origin` allowlist; required in session mode, empty allows all in anonymous mode |
| `CHATSTER_AUTH_MODE` | `anonymous` | Authentication boundary: public display-name demo or required signed sessions |
| `CHATSTER_SESSION_SECRET` | _(required in session mode)_ | HMAC signing key of at least 32 bytes; never logged |
| `CHATSTER_AUTH_USERS_JSON` | _(required in session mode)_ | Secret 32-512 byte token-to-user records with at most 64 room grants; never logged |
| `CHATSTER_SESSION_TTL` | `1h` | Session lifetime, accepted range `1m` through `24h` |
| `CHATSTER_SESSION_COOKIE_SECURE` | `true` | Set `false` only for local HTTP session testing |
| `CHATSTER_WS_UPGRADE_RPS` | `5` | Per-IP WebSocket upgrades per second (`0` disables) |
| `CHATSTER_WS_UPGRADE_BURST` | `10` | Burst size for the upgrade limiter |
| `CHATSTER_MESSAGE_RPS` | `5` | Per-client chat messages per second (`0` disables) |
| `CHATSTER_MESSAGE_BURST` | `10` | Burst size for the message limiter |
| `CHATSTER_MESSAGE_RETENTION_DAYS` | `0` | Delete persisted messages older than this many days at startup (`0` disables) |
| `CHATSTER_AUDIT_RETENTION_DAYS` | `0` | Delete moderation audit events older than this many days at startup (`0` disables) |

## Storage modes

SQLite remains the default for local development, Docker Compose, and the checked-in Render
Blueprint. Keep `CHATSTER_DB_PATH` on a persistent volume in any environment where history
must survive a restart or redeploy.

Postgres is explicit and has no fallback:

```bash
CHATSTER_STORAGE=postgres \
CHATSTER_POSTGRES_DSN='postgres://chatster:${CHATSTER_POSTGRES_PASSWORD}@db.example.com:5432/chatster?sslmode=verify-full' \
CHATSTER_POSTGRES_MIN_CONNS=2 \
CHATSTER_POSTGRES_MAX_CONNS=10 \
go run .
```

The process fails before serving traffic when the DSN is missing, the pool bounds are invalid,
the initial ping fails, or migrations fail. It never creates a replacement SQLite database.
Production Postgres connections should use `sslmode=verify-full` with a trusted CA and a
secret manager or environment injection for credentials. `sslmode=disable` is for local CI or
development only.

## Health checks

`GET /health` returns JSON:

- **200** — `status` is `ok`, `database` is `ok`.
- **503** — `status` is `degraded` when the database ping fails within ~2s.

Example:

```json
{"status":"ok","database":"ok","service":"chatster"}
```

Use this endpoint for Kubernetes liveness/readiness or load balancer probes.

## Session operations

`GET /api/session` reports the current auth mode and browser session state without returning
credentials. In session mode, `POST /api/session` exchanges an operator-provisioned bearer
token for the signed cookie, and `DELETE /api/session` logs out by expiring that cookie.
History and WebSocket room access return `401` for missing, invalid, or expired sessions and
`403` when the signed principal lacks the requested room grant.

Sessions are stateless. Removing an access token prevents new logins but does not revoke an
already issued cookie. Rotate `CHATSTER_SESSION_SECRET` to revoke every session immediately,
or wait for the configured short TTL. A server-side session store is required for selective
immediate revocation. Cookie signing secrets and provisioned tokens must stay in the platform
secret manager and out of logs, diagnostics, and shell history.
Configure edge and reverse-proxy access logs to redact `Authorization` and `Cookie` headers.

## Metrics

`GET /metrics` exposes **Prometheus** text format (Go runtime collectors plus `chatster_*` counters/gauges). Scrape from your Prometheus server or agent; see [OBSERVABILITY.md](OBSERVABILITY.md) for naming and SLO guidance.

## Logging

The API emits **structured JSON logs** on stdout via `log/slog` (Go 1.22+). Aggregate with your log stack (CloudWatch, Loki, Datadog, etc.).

## Process lifecycle

The server handles **SIGINT** and **SIGTERM** with a shared **30s** shutdown budget:

1. The WebSocket hub enters draining mode and rejects new upgrades with **503**.
2. Existing WebSocket clients receive close code **1012** (`service restart`) and reason `server shutting down`.
3. The hub waits for client read loops and stops its goroutine, then the HTTP server completes graceful shutdown.

Monitor `chatster_websocket_drain_duration_seconds`,
`chatster_websocket_drain_clients_remaining`, and
`chatster_websocket_drain_forced_closes_total`. A non-zero forced-close count means the
process reached the deadline with at least one client loop still active; investigate
network backpressure or a stuck handler before increasing the shutdown budget.

## Docker

### Local two-service compose

From the repository root:

```bash
docker compose up --build
```

- API: [http://localhost:8080](http://localhost:8080) (WebSocket: `ws://localhost:8080/ws?room=general`)
- Web UI: [http://localhost:3000](http://localhost:3000) (static build; browser connects to the selected room, default `ws://localhost:8080/ws?room=general`)

SQLite data persists in the **`chatster-data`** Docker volume.

### Production single-service image

The root [Dockerfile](../Dockerfile) builds the React app and Go backend into one container. The Go server serves the React build from `CHATSTER_STATIC_DIR`, so browser HTTP, API calls, and WebSockets share one origin.

```bash
docker build -t chatster:prod .
docker run --rm \
  -p 8080:8080 \
  -v chatster-data:/data \
  -e CHATSTER_ALLOWED_ORIGINS=http://localhost:8080 \
  chatster:prod
```

- UI/API: [http://localhost:8080](http://localhost:8080)
- Health: [http://localhost:8080/health](http://localhost:8080/health)
- WebSocket: `ws://localhost:8080/ws?room=general`

## Persistence

Startup applies ordered migrations for the selected backend and records them in
`schema_migrations`. SQLite applies them on its single connection; Postgres takes a database
advisory lock while applying each version. Check the migration ledger before serving real
traffic after a schema change.

When `CHATSTER_MESSAGE_RETENTION_DAYS` is positive, startup deletes older rows from `messages`, logs the deleted count, and increments `chatster_chat_messages_pruned_total`. When `CHATSTER_AUDIT_RETENTION_DAYS` is positive, the same lifecycle applies to `moderation_audit_log` and `chatster_moderation_audit_events_pruned_total`. The cleanup is startup-based and runs against whichever repository is selected.

## Backups

Copy the SQLite file while the process is stopped, or use SQLite’s backup API / `.backup` for online copies if you extend the service.

For Postgres, use the platform’s managed backup/PITR facility or a tested `pg_dump`/restore
process. Migrations are forward-only and startup-owned; verify the target backup by restoring
it into a disposable database before a production rollback.

## Production hardening (checklist)

- Terminate TLS at the edge; use **`wss://`** for WebSockets.
- For Postgres, require TLS hostname verification (`sslmode=verify-full`) and keep the CA,
  username, and password outside committed files and logs.
- Set **`CHATSTER_ALLOWED_ORIGINS`** to match your static app origins (see [THREAT_MODEL.md](THREAT_MODEL.md)).
- Keep session cookies `Secure`, use short expiries, and rotate provisioned tokens and the signing secret through the host's secret manager.
- Run the API as a non-root user (Dockerfile already uses a dedicated user).
- Monitor `/health`, **`/metrics`**, log volume, and DB disk growth.
