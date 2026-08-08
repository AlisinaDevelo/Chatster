---
id: 0002
title: Deploy and verify the public Render demo
status: in-progress
agent: release-engineer
model: sonnet
depends_on: []
parent: 0001
---

## Goal
Turn the checked-in Render Blueprint into a real public demo and publish the URL with honest single-node SQLite caveats. This is the highest-signal remaining proof item and requires the owner to connect the repository to Render.

## Acceptance criteria
- [ ] `render.yaml` deploys from `main` as one web service with the persistent `/data` disk and the configured health check.
- [ ] `CHATSTER_ALLOWED_ORIGINS` is set to the exact deployed HTTPS origin and no credentials or secret values enter the repository.
- [ ] The deployed origin returns healthy responses for `/health`, `/rooms/general`, `/rooms/engineering`, `/metrics`, and room-filtered `/api/messages`.
- [ ] Two browser tabs verify same-room delivery, cross-room isolation, reconnect history, and a clean reload.
- [ ] `README.md` and `docs/DEPLOYMENT.md` publish the live URL, deploy date, smoke command, and SQLite/in-memory-hub limitations.

## Context
Use `render.yaml` and `docs/DEPLOYMENT.md`. Keep `numInstances: 1` while storage is SQLite and hub state is in memory. The current CI production smoke proves the container locally but cannot prove an external deployment.

## Notes
Platform references: https://render.com/docs/web-services, https://render.com/docs/blueprint-spec, and https://render.com/docs/disks.
- Added `scripts/smoke-deployment.sh` for health, room-route, metrics, and room-filtered history checks.
- Added `CHATSTER_E2E_BASE_URL` support so the existing Chromium smoke can target a deployed origin without starting local servers.
- The production-image CI job now runs the same HTTP smoke script used for post-deploy verification.
- Local HTTP smoke, Go race tests, frontend tests/build, and the full local Chromium suite pass.
- The external Render deployment remains pending because this workspace has no Render account, service, CLI, or deployment credentials; no public URL is claimed yet.
