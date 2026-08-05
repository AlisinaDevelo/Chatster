---
id: 0003
title: Add Playwright browser-matrix smoke coverage
status: review
agent: test-engineer
model: sonnet
depends_on: []
parent: 0001
---

## Goal
Add a real browser-level smoke path that proves the user-facing chat workflow, room routing, accessibility-sensitive controls, and reconnect behavior beyond jsdom tests.

## Acceptance criteria
- [x] Add a pinned `@playwright/test` setup with a documented local command and deterministic backend/frontend `webServer` configuration.
- [x] Cover username onboarding, same-room message delivery, switching to `engineering`, cross-room isolation, and refresh/reconnect history.
- [x] Run a fast Chromium project on every CI push; document or schedule the Firefox and WebKit matrix without making the default pipeline flaky.
- [x] Capture trace, screenshot, and video artifacts on failure; avoid arbitrary sleeps in favor of Playwright assertions and events.
- [x] Keep the accessibility smoke and browser smoke responsibilities distinct and document the remaining device-matrix limits.

## Context
The React tests cover components and mocked API calls, while the repo previously had a successful manual browser smoke. Start from `frontend/`, `docs/FRONTEND.md`, and `docs/WORKFLOWS.md`; preserve the existing port 3000/8080 development contract.

## Notes
Research anchor: https://playwright.dev/docs/intro. Playwright Test supports browser projects, isolation, parallel execution, and trace/report tooling.
