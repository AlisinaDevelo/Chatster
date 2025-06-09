# Chatster

Real-time chat reference stack: **Go** WebSocket hub + **SQLite** history, **React** client, **Docker**-ready, **CI** with lint and coverage.

[![CI](https://github.com/AliSinaDevelo/Chatster/actions/workflows/ci.yml/badge.svg)](https://github.com/AliSinaDevelo/Chatster/actions/workflows/ci.yml)

## Highlights

- WebSocket broadcast with reconnect and clean teardown on navigation (React Strict Mode safe).
- Last **50** messages replayed on connect; new messages persisted with timestamps.
- **`GET /health`** with **SQLite ping** (503 when degraded) for real probes.
- **Structured JSON logs** (`slog`) and **graceful shutdown** on SIGINT/SIGTERM.
- **GitHub Actions**: golangci-lint, `go test -race` + coverage artifact, ESLint, Jest, production build.
- **Dependabot** for Actions, Go, and npm.
- **Docker Compose** for a one-command demo stack.

## Quick start

### Option A — Docker (fastest to see the UI)

```bash
docker compose up --build
```

Open **http://localhost:3000** (UI) and **http://localhost:8080/health** (API health).

### Option B — Native (best for development)

**Terminal 1 — API**

```bash
cd backend && go run .
```

**Terminal 2 — React**

```bash
cd frontend && npm install && npm start
```

Open **http://localhost:3000**. Use two browser tabs or windows to test live messaging.

## Configuration

| Variable | Scope | Purpose |
|----------|--------|---------|
| `CHATSTER_HTTP_ADDR` | Backend | Listen address (default `:8080`). |
| `CHATSTER_DB_PATH` | Backend | SQLite file (default `./chatster.db`). |
| `REACT_APP_WS_URL` | Frontend build | Full WebSocket URL (production / Docker build args). |
| `REACT_APP_WS_PORT` | Frontend dev | Backend port when using default dev WebSocket URL. |

Copy `frontend/.env.example` to `frontend/.env.local` when overriding the client.

## Scripts

| Command | Description |
|---------|-------------|
| `make test` | Backend tests + frontend tests (CI mode). |
| `make lint` | golangci-lint + ESLint (requires golangci-lint installed locally). |
| `make docker-up` | `docker compose up --build`. |
| `cd backend && go test -race ./...` | Go tests only. |
| `cd frontend && npm run test:ci` | Jest once. |
| `cd frontend && npm run build` | Optimized static build. |

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — components, data flow, security notes.
- [Workflows](docs/WORKFLOWS.md) — CI, Dependabot, local and Docker dev.
- [Operations](docs/OPERATIONS.md) — health checks, logging, production checklist.
- [Contributing](CONTRIBUTING.md) — PRs, `make lint`, code of conduct.

## Stack

Go 1.22 · Gorilla Mux & WebSocket · SQLite (CGO) · React 18 · Sass · Docker · GitHub Actions.

## License

See [LICENSE](LICENSE).
