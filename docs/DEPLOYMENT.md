# Deployment

Chatster can deploy as a single container from the root [Dockerfile](../Dockerfile). The image builds the React frontend, builds the Go backend, and serves both from one process. It listens on `CHATSTER_HTTP_ADDR` when set, otherwise it uses a numeric platform `PORT` and falls back to `:8080`.

## Build locally

```bash
docker build -t chatster:prod .
```

Run it with a persistent SQLite volume:

```bash
docker run --rm \
  -p 8080:8080 \
  -v chatster-data:/data \
  -e CHATSTER_ALLOWED_ORIGINS=http://localhost:8080 \
  chatster:prod
```

Then open:

- UI/API: <http://localhost:8080>
- Health: <http://localhost:8080/health>
- Metrics: <http://localhost:8080/metrics>

## Host requirements

- Container runtime with WebSocket support.
- Public HTTPS endpoint that forwards WebSocket upgrades.
- Persistent writable disk mounted at `/data` when using SQLite.
- A managed Postgres instance and secret injection when using Postgres mode.
- Health check path: `/health`.
- Optional Prometheus scrape path: `/metrics`.

## Required production env

| Variable | Example | Why |
|----------|---------|-----|
| `CHATSTER_HTTP_ADDR` | `:8080` | Optional explicit listen address; unset uses the numeric `PORT` supplied by a hosting platform. |
| `CHATSTER_STORAGE` | `sqlite` | Select `sqlite` (default) or `postgres`; selection is explicit and has no fallback. |
| `CHATSTER_DB_PATH` | `/data/chatster.db` | Keep SQLite on persistent storage; ignored by Postgres mode. |
| `CHATSTER_POSTGRES_DSN` | `postgres://...?...` | Secret DSN required for Postgres mode; use `sslmode=verify-full` in production. |
| `CHATSTER_POSTGRES_MIN_CONNS` / `CHATSTER_POSTGRES_MAX_CONNS` | `2` / `10` | Size the Postgres connection pool for the instance. |
| `CHATSTER_STATIC_DIR` | `/app/static` | Already set by the production image. |
| `CHATSTER_ALLOWED_ORIGINS` | `https://chatster.example.com` | Restrict browser WebSocket origins. |
| `CHATSTER_WS_UPGRADE_RPS` | `5` | Per-IP upgrade abuse control. |
| `CHATSTER_MESSAGE_RPS` | `5` | Per-client message abuse control. |
| `CHATSTER_MESSAGE_RETENTION_DAYS` | `30` | Optional startup cleanup for old chat history. |
| `CHATSTER_AUDIT_RETENTION_DAYS` | `90` | Optional startup cleanup for moderation audit history. |

## Postgres mode

Use Postgres when the service needs shared durable storage across instances or has outgrown
SQLite’s single-file writer. Configure it with secret-managed environment variables:

```text
CHATSTER_STORAGE=postgres
CHATSTER_POSTGRES_DSN=postgres://chatster:<secret>@<host>:5432/chatster?sslmode=verify-full
CHATSTER_POSTGRES_MIN_CONNS=2
CHATSTER_POSTGRES_MAX_CONNS=10
```

Chatster pings the pool and applies versioned migrations before it serves HTTP. A missing DSN,
TLS/connectivity failure, invalid pool bounds, or migration error is fatal; the process does
not silently fall back to SQLite. Use a trusted CA and verify the hostname. Keep the DSN out of
`render.yaml`, image layers, logs, and committed `.env` files. Back up Postgres with managed
PITR or a tested `pg_dump`/restore procedure.

Postgres makes durable history shareable, but the WebSocket hub remains process-local. Multiple
instances still need the Redis/event-fanout work described in [SCALING.md](SCALING.md) before
clients on different instances can see the same live room broadcasts.

## Platform notes

- Use one instance while Chatster uses SQLite and an in-memory WebSocket hub.
- Attach a persistent disk/volume before accepting real traffic; without it, chat history and moderation audit rows disappear on redeploy.
- Terminate TLS at the platform edge; the browser should use `https://` and `wss://`.
- If your platform injects a numeric dynamic port through `PORT`, Chatster uses it automatically when `CHATSTER_HTTP_ADDR` is unset. Set `CHATSTER_HTTP_ADDR` explicitly only when the platform uses a different address convention.
- Scale-out requires the design in [SCALING.md](SCALING.md): shared storage plus sticky sessions or pub/sub fanout.

## Render Blueprint

The checked-in [render.yaml](../render.yaml) provisions a Docker web service from the root image, a `/health` HTTP check, a one-instance deployment, and a 1 GB persistent disk mounted at `/data` for SQLite. The service uses Render's numeric `PORT` automatically; no hardcoded Render port is required.

To deploy it:

1. In Render, create a new Blueprint and connect the `AlisinaDevelo/Chatster` repository.
2. Keep the `main` branch and review the proposed `chatster-public-demo` web service.
3. Provide `CHATSTER_ALLOWED_ORIGINS` as the exact HTTPS origin Render assigns to the service, such as `https://chatster-public-demo.onrender.com`.
4. Deploy, then verify the service URL with `curl -fsS https://<service>.onrender.com/health`.

This Blueprint intentionally uses the paid `starter` plan because Render persistent disks are available to paid web services. The disk is single-instance storage, so keep `numInstances: 1` while Chatster uses SQLite and an in-memory WebSocket hub. See Render's [Web Services](https://render.com/docs/web-services), [Blueprint reference](https://render.com/docs/blueprint-spec), and [Persistent Disks](https://render.com/docs/disks) documentation for the platform contract and current pricing/limits.

The repository does not claim a live demo URL until this Blueprint is actually connected to a Render account and deployed.

## Smoke after deploy

```bash
curl -fsS https://chatster.example.com/health
```

Open two browser tabs, join with different display names, and send a message. Then check `/metrics` for:

- `chatster_websocket_upgrades_total{result="ok"}`
- `chatster_chat_messages_ingested_total`
- `chatster_chat_message_persist_duration_seconds_bucket`

## Rollback

Deploy the previous image tag. Keep the `/data` volume attached to the replacement container. Current migrations are additive; do not delete the SQLite file unless you intentionally want a fresh demo history.
