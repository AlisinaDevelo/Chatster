# Chatster frontend

Create React App shell for the Chatster WebSocket client.

- **`npm start`** — dev server ([http://localhost:3000](http://localhost:3000)); expects the API on `ws://127.0.0.1:8080/ws` unless overridden via env (see `.env.example`).
- **Docker** — production-style image with nginx: `docker compose up --build` from the repo root (see root [README.md](../README.md)).
- **`npm run test`** / **`npm run test:ci`** — Jest.
- **`npm run build`** — static assets for deployment.

Project-wide docs live in the repository root [README.md](../README.md).
