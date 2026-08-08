---
id: 0001
title: Close the public-proof and scale-ready backlog
status: done
agent: architect
model: opus
depends_on: []
---

## Goal
Create a visible, verifiable next phase for Chatster after the room-scoped release. The epic covers public deployment proof, browser-level confidence, graceful operations, and carefully staged scale extensions.

## Acceptance criteria
- [x] Child tasks exist for public deployment, browser QA, shutdown drain, tracing, storage, cross-instance fan-out, authentication, and long-history performance.
- [x] Child task dependencies are represented in GitHub and match the local Forge ledger.
- [x] The improvement plan names one immediate execution order and separates optional platform tracks from the public-demo finish line in `docs/ROADMAP.md`.

## Context
Chatster is currently green on `main` with room-scoped SQLite history, WebSocket isolation, CI load smoke, production-image smoke, and a Render Blueprint. There are no existing GitHub issues, so this issue is the parent for the new managed backlog.

## Notes
Research anchors: https://playwright.dev/docs/intro, https://opentelemetry.io/docs/languages/go/, https://redis.io/docs/latest/develop/pubsub/, and https://www.postgresql.org/docs/current/libpq-connect.html.

## Verification
- Forge status reports 11 local tasks and 11 managed GitHub tasks with no sync operations pending.
- `docs/ROADMAP.md` records the Render deployment finish line and the gated authentication, Redis, and Postgres tracks.
