# Frontend engineering notes

The UI is a **Vite + React** SPA: strong focus on **clarity, accessibility, and real-time UX**. **Backend and platform engineering are the primary showcase** for this repository; the client is deliberately small and readable.

## Positioning

| Area | Stance |
|------|--------|
| **State** | Local React state + WebSocket callbacks—appropriate for a single-screen chat without global client stores. |
| **Data fetching** | WebSocket events plus the REST history endpoint for initial load and reconnect catch-up. |
| **Build tooling** | Vite 8 with the output directory kept at `build/` for the Go server and container image. |

## Accessibility (a11y)

Implemented practices include:

- **Landmarks:** `header` / `main` / `footer`, labeled chat **region** (`aria-labelledby`).
- **Live region:** message list uses `role="log"` and `aria-live="polite"` so assistive tech is notified of new messages (tunable if volume becomes noisy).
- **Forms:** visible labels (or visually hidden where design uses placeholders), `aria-describedby` for hints, submit disabled when disconnected.
- **Motion:** global `prefers-reduced-motion` respected in styles (see `index.css` / component SCSS).
- **Automated checks:** the rendered app runs an axe-core accessibility smoke test in `App.test.jsx`, which executes in the normal Vitest/CI path.

**Gaps / next steps:** full keyboard path for custom scroll containers; optional “reduce live announcements” setting for high-traffic rooms.

## Performance budget (guidance)

| Budget | Target | Notes |
|--------|--------|-------|
| First load (dev) | N/A | Vite provides fast native-module development feedback. |
| Production JS (gzip) | Keep main chunk **< 200 KB** where practical | Current build is small; re-check after adding libraries. |
| Re-renders | O(n) list append | For very long sessions, **virtualize** the message list (`react-window` / similar). |

## Testing

- **Vitest + Testing Library** for components and mocked WebSocket `api` module.
- **E2E** (Playwright/Cypress) is a documented extension in [WORKFLOWS.md](WORKFLOWS.md)—not required for unit coverage.

## Security (client)

- WebSocket URL from **`VITE_WS_URL`** in production builds—avoid hardcoding internal hosts.
- No secrets in the bundle; treat usernames as **non-sensitive display names** unless you add real auth.
