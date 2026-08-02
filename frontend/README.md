# Chatster frontend

Vite + React shell for the Chatster WebSocket client.

- **`npm start`** — dev server ([http://localhost:3000](http://localhost:3000)); expects the API on `ws://127.0.0.1:8080/ws` unless overridden via env (see `.env.example`).
- **Docker** — production-style image with nginx: `docker compose up --build` from the repo root (see root [README.md](../README.md)).
- **`npm run test`** / **`npm run test:ci`** — Vitest.
- **`npm run build`** — static assets for deployment.
- **`npm run preview`** — serve the production build locally.

The Vite build writes to `build/` to match the Go server and container deployment paths. `VITE_*` environment variables are canonical; the client also accepts the previous `REACT_APP_*` names for existing local builds.

Project-wide docs live in the repository root [README.md](../README.md).
