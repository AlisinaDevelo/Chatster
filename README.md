# Chatster

Real-time chat demo: **Go** WebSocket hub + **SQLite** history, **React** client with a dark glass UI.

[![CI](https://github.com/AliSinaDevelo/Chatster/actions/workflows/ci.yml/badge.svg)](https://github.com/AliSinaDevelo/Chatster/actions/workflows/ci.yml)

## Highlights

- WebSocket broadcast with reconnect and clean teardown on navigation (Strict Mode safe).
- Last **50** messages replayed on connect; new messages persisted with timestamps.
- **`GET /health`** for load balancers and monitoring.
- **GitHub Actions** runs Go tests (race + vet) and React tests + production build.

## Quick start

**Backend** (port `8080`, creates `chatster.db` in `backend/`):

```bash
cd backend && go run .
```

**Frontend**:

```bash
cd frontend && npm install && npm start
```

Open [http://localhost:3000](http://localhost:3000), pick a username, then chat. Run two browser windows to see live delivery.

## Configuration

| Variable | Purpose |
|----------|---------|
| `REACT_APP_WS_URL` | Full WebSocket URL (e.g. `wss://api.example.com/ws`). |
| `REACT_APP_WS_PORT` | Dev-only port if the API is not on `8080`. |

Copy `frontend/.env.example` to `frontend/.env.local` when needed.

## Scripts

| Command | Description |
|---------|-------------|
| `make test` | Backend `go test -race` + frontend `npm run test:ci`. |
| `cd backend && go test -race ./...` | Go unit tests. |
| `cd frontend && npm run test:ci` | Jest once (CI). |
| `cd frontend && npm run build` | Optimized static build. |

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — data flow, components, security notes.
- [Workflows](docs/WORKFLOWS.md) — CI, local dev, release checklist.

## Stack

Go 1.22 · Gorilla Mux & WebSocket · SQLite (CGO) · React 18 · Sass · GitHub Actions.

## License

See [LICENSE](LICENSE).
