# Deployment

Chatster can deploy as a single container from the root [Dockerfile](../Dockerfile). The image builds the React frontend, builds the Go backend, and serves both from one process on `:8080`.

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
- Persistent writable disk mounted at `/data` for SQLite.
- Health check path: `/health`.
- Optional Prometheus scrape path: `/metrics`.

## Required production env

| Variable | Example | Why |
|----------|---------|-----|
| `CHATSTER_HTTP_ADDR` | `:8080` | Listen on the platform-provided container port. |
| `CHATSTER_DB_PATH` | `/data/chatster.db` | Keep SQLite on persistent storage. |
| `CHATSTER_STATIC_DIR` | `/app/static` | Already set by the production image. |
| `CHATSTER_ALLOWED_ORIGINS` | `https://chatster.example.com` | Restrict browser WebSocket origins. |
| `CHATSTER_WS_UPGRADE_RPS` | `5` | Per-IP upgrade abuse control. |
| `CHATSTER_MESSAGE_RPS` | `5` | Per-client message abuse control. |

## Platform notes

- Use one instance while Chatster uses SQLite and an in-memory WebSocket hub.
- Attach a persistent disk/volume before accepting real traffic; without it, chat history and moderation audit rows disappear on redeploy.
- Terminate TLS at the platform edge; the browser should use `https://` and `wss://`.
- If your platform injects a dynamic port through `PORT`, set `CHATSTER_HTTP_ADDR=:$PORT` in that platform's env syntax or entrypoint.
- Scale-out requires the design in [SCALING.md](SCALING.md): shared storage plus sticky sessions or pub/sub fanout.

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
