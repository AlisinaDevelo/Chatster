# Workflows

How the repository is validated and how to work on it day to day.

## Continuous integration

GitHub Actions workflow **`.github/workflows/ci.yml`** runs on every push and pull request to `main`. It uses **concurrency** so newer pushes cancel stale runs on the same branch.

| Job | What it does |
|-----|----------------|
| **backend** | `golangci-lint`, `go test -race` with **coverage**, `go vet`, tiny WebSocket load smoke |
| **frontend** | `npm ci`, `npm run lint`, `npm run test:ci`, `npm run build` |
| **production-image** | Builds the root Docker production image and smokes the running single-container app |

The backend job uploads a **`backend-runtime-proof`** artifact containing:

- `coverage.out` — raw Go coverage profile.
- `coverage-summary.txt` — human-readable `go tool cover -func` summary.
- `wsload-smoke.json` — 4-client / 3-message WebSocket fanout smoke result with `-fail-on-loss`.
- `server-smoke.log` — backend log from the smoke run.

Requirements: Go **1.22**, Node **20**, [golangci-lint](https://golangci-lint.run/) config at **`.golangci.yml`** (repo root), and a lockfile (`frontend/package-lock.json`) in sync with `package.json`.

The production image job waits for `/health`, then checks the React shell at `/`, SPA fallback at `/rooms/general`, empty history from `/api/messages?limit=1`, and Prometheus output from `/metrics`.

## Dependency updates

[Dependabot](https://docs.github.com/en/code-security/dependabot) is configured in **`.github/dependabot.yml`** for GitHub Actions, Go modules, and npm (weekly batches, labeled `dependencies`).

## Local checks

From the repository root:

```bash
make test
make lint
```

`make lint` requires **golangci-lint** on your PATH for the Go package (install via [official docs](https://golangci-lint.run/welcome/install/) or rely on CI).

Or individually:

```bash
make test-backend   # go test -race ./...
make test-frontend  # npm run test:ci
make lint-backend   # golangci-lint run ./...
make lint-frontend  # npm run lint
make build-frontend # production build
```

## Local development (native)

**Terminal 1 — API and WebSocket**

```bash
cd backend && go run .
```

Server listens on **`:8080`** by default and writes **`chatster.db`** in the current working directory unless `CHATSTER_DB_PATH` is set.

**Terminal 2 — React (development)**

```bash
cd frontend && npm install && npm start
```

The dev client targets **`ws://127.0.0.1:8080/ws`** by default. If the backend uses another port, set `REACT_APP_WS_PORT` or `REACT_APP_WS_URL` (see `frontend/.env.example`).

Open [http://localhost:3000](http://localhost:3000) to see the UI.

## Local development (Docker)

```bash
docker compose up --build
```

- UI: [http://localhost:3000](http://localhost:3000)
- API: [http://localhost:8080/health](http://localhost:8080/health)

See [OPERATIONS.md](OPERATIONS.md) for health semantics and persistence.

## Release checklist (manual)

1. `make test` and `make lint`
2. `cd frontend && npm run build` — serve `frontend/build` behind HTTPS in production; use `wss://` for WebSockets.
3. `docker build -t chatster:prod .` — mirrors the CI production-image check.
4. Run the Go binary (or container) with a writable directory for SQLite; set `CHATSTER_DB_PATH` explicitly in production.

## Branching and commits

- Prefer small, focused commits with short subject lines.
- Open a PR to run CI before merging to `main`.
- Use the [pull request template](../.github/pull_request_template.md) and linked issue when applicable.

## Further reading

- [Scaling & failure modes](SCALING.md)
- [Threat model](THREAT_MODEL.md)
- [Observability](OBSERVABILITY.md)
- [Frontend engineering](FRONTEND.md)
- [Non-goals](NON_GOALS.md)
- [ADR index](adr/README.md)
