# Frontend engineering notes

The UI is a **Create React App** SPA: strong focus on **clarity, accessibility, and real-time UX**, not on replacing CRA with a meta-framework. **Backend and platform engineering are the primary showcase** for this repository; the client is deliberately small and readable.

## Positioning

| Area | Stance |
|------|--------|
| **State** | Local React state + WebSocket callbacks—appropriate for a single-screen chat without global client stores. |
| **Data fetching** | WebSocket-only; no REST client layer needed for the demo. |
| **Build tooling** | CRA 5 / `react-scripts`—stable but legacy; migrating to **Vite** would be a separate ADR if pursued. |

## Accessibility (a11y)

Implemented practices include:

- **Landmarks:** `header` / `main` / `footer`, labeled chat **region** (`aria-labelledby`).
- **Live region:** message list uses `role="log"` and `aria-live="polite"` so assistive tech is notified of new messages (tunable if volume becomes noisy).
- **Forms:** visible labels (or visually hidden where design uses placeholders), `aria-describedby` for hints, submit disabled when disconnected.
- **Motion:** global `prefers-reduced-motion` respected in styles (see `index.css` / component SCSS).

**Gaps / next steps:** full keyboard path for custom scroll containers; optional “reduce live announcements” setting for high-traffic rooms; automated **axe** checks in CI.

## Performance budget (guidance)

| Budget | Target | Notes |
|--------|--------|-------|
| First load (dev) | N/A | CRA dev server is unoptimized by design. |
| Production JS (gzip) | Keep main chunk **< 200 KB** where practical | Current build is small; re-check after adding libraries. |
| Re-renders | O(n) list append | For very long sessions, **virtualize** the message list (`react-window` / similar). |

## Testing

- **Jest + Testing Library** for components and mocked WebSocket `api` module.
- **E2E** (Playwright/Cypress) is a documented extension in [WORKFLOWS.md](WORKFLOWS.md)—not required for unit coverage.

## Security (client)

- WebSocket URL from **`REACT_APP_WS_URL`** in production builds—avoid hardcoding internal hosts.
- No secrets in the bundle; treat usernames as **non-sensitive display names** unless you add real auth.
