# Frontend engineering notes

The UI is a **Vite + React** SPA: strong focus on **clarity, accessibility, and real-time UX**. **Backend and platform engineering are the primary showcase** for this repository; the client is deliberately small and readable.

## Positioning

| Area | Stance |
|------|--------|
| **State** | Local React state + WebSocket callbacks—appropriate for a single-screen chat without global client stores. The active room is mirrored in `/rooms/<name>`. |
| **Data fetching** | Room-aware WebSocket events plus `GET /api/messages?room=...` for initial load and reconnect catch-up. |
| **Build tooling** | Vite 8 with the output directory kept at `build/` for the Go server and container image. |

## Accessibility (a11y)

Implemented practices include:

- **Landmarks:** `header` / `main` / `footer`, labeled chat **region** (`aria-labelledby`).
- **Live region:** message list uses `role="log"` and `aria-live="polite"` so assistive tech is notified of new messages (tunable if volume becomes noisy).
- **Keyboard scrolling:** the scrollable message log is tabbable and exposes a visible focus ring, so keyboard users can enter and scroll the history directly.
- **Announcement preference:** the **Quiet updates** checkbox persists locally and switches the message log to `aria-live="off"` for high-traffic rooms.
- **Room navigation:** the header exposes a labeled native select for the active room, with URL state and reconnect/history behavior kept in sync.
- **Forms:** visible labels (or visually hidden where design uses placeholders), `aria-describedby` for hints, submit disabled when disconnected.
- **Motion:** global `prefers-reduced-motion` respected in styles (see `index.css` / component SCSS).
- **Automated checks:** the rendered app runs an axe-core accessibility smoke test in `App.test.jsx`, which executes in the normal Vitest/CI path. Playwright separately covers the real-browser chat workflow; it does not replace the accessibility check.

**Next steps:** validate the interaction across a browser/device matrix and consider list virtualization for very long sessions.

## Performance budget (guidance)

| Budget | Target | Notes |
|--------|--------|-------|
| First load (dev) | N/A | Vite provides fast native-module development feedback. |
| Production JS (gzip) | Keep main chunk **< 200 KB** where practical | Current build is small; re-check after adding libraries. |
| Re-renders | O(n) list append for short histories; virtual window for histories with **1,000+ messages** | Keep the threshold tied to the measured baseline below. |

## Long-history baseline

The history list was measured in local Chromium with the production `ChatHistory`
markup, representative usernames/content/timestamps, and a 374px scroll viewport.
React Profiler commit time, DOM size, and the browser's exposed JS heap counter were
sampled before choosing the virtualization dependency. Heap values are directional
because the browser may collect between samples.

| Messages | Mount commit | Message rows / descendants | Heap sample | Scroll range |
|---:|---:|---:|---:|---:|
| 100 | 12 ms | 100 / 601 | 28 MB | 8,270 px |
| 500 | 29 ms | 500 / 3,001 | 31 MB | 41,348 px |
| 1,000 | 52 ms | 1,000 / 6,001 | 40 MB | 82,695 px |
| 2,500 | 148 ms | 2,500 / 15,001 | 55 MB | 206,738 px |
| 5,000 | 294 ms | 5,000 / 30,001 | 52 MB | 413,477 px |

The **1,000-message threshold** keeps normal room history on the simple DOM path while
avoiding the measured 148–294 ms large-history mount cost. At or above the threshold,
`@tanstack/react-virtual` renders a small overscanned window, measures variable-height
rows, preserves the same message markup, and scrolls the latest item into view. The
dependency is headless and adds no styling; the tradeoff is a small amount of positioning
and measurement logic in `ChatHistory` in exchange for bounded DOM work. The current
history API still requests 50 records, so this path primarily protects long-lived live
rooms and future pagination.

## Testing

- **Vitest + Testing Library** for components, room routing, room-aware history requests, and mocked WebSocket `api` module.
- Long-history component coverage verifies the virtual window still exposes a tabbable
  `role="log"`, rendered message content, timestamps, and ownership styling.
- **Browser workflow smoke:** `npm run test:e2e` runs Chromium, Firefox, and WebKit locally after `npx playwright install`. CI runs the Chromium project on every push and stores the HTML report plus failure trace/screenshot/video artifacts. The matrix currently covers desktop engines only; mobile browsers, real assistive technology, and device farms remain outside this smoke path.

## Security (client)

- WebSocket URL from **`VITE_WS_URL`** in production builds—avoid hardcoding internal hosts.
- No secrets in the bundle; treat usernames as **non-sensitive display names** unless you add real auth.
