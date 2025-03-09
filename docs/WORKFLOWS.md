# Workflows

How the repository is validated and how to work on it day to day.

## Continuous integration

GitHub Actions workflow **`.github/workflows/ci.yml`** runs on every push and pull request to `main`:

| Job | What it does |
|-----|----------------|
| **backend** | `go test -race ./...` and `go vet ./...` in `backend/` |
| **frontend** | `npm ci`, `npm run test:ci`, `npm run build` in `frontend/` |

Requirements: Go **1.22**, Node **20**, and a lockfile (`frontend/package-lock.json`) in sync with `package.json`.

## Local checks

From the repository root:

```bash
make test
```

Or individually:

```bash
make test-backend   # go test -race ./...
make test-frontend  # npm run test:ci
make build-frontend # production build
```

## Local development

**Terminal 1 — API and WebSocket**

```bash
cd backend && go run .
```

Server listens on **`:8080`** and writes **`chatster.db`** in the current working directory.

**Terminal 2 — React (development)**

```bash
cd frontend && npm install && npm start
```

The dev client targets **`ws://127.0.0.1:8080/ws`** by default. If the backend uses another port, set `REACT_APP_WS_PORT` or `REACT_APP_WS_URL` (see `frontend/.env.example`).

## Release checklist (manual)

1. `make test`
2. `cd frontend && npm run build` — serve `frontend/build` behind HTTPS in production; use `wss://` for WebSockets.
3. Run the Go binary with a writable directory for SQLite (or configure a dedicated DSN if you extend `db.Open`).

## Branching and commits

- Prefer small, focused commits with short subject lines.
- Open a PR to run CI before merging to `main`.
