---
id: 0006
title: Define the storage repository boundary for Postgres mode
status: review
agent: architect
model: opus
depends_on: []
parent: 0001
---

## Goal
Design a narrow storage contract that keeps SQLite as the default while making a future Postgres implementation an explicit, testable mode rather than a rewrite hidden inside handlers.

## Acceptance criteria
- [x] Add an ADR that defines interfaces for recent room history, message persistence, audit events, health checks, retention, and migration/startup errors.
- [x] Specify ordering, timestamp, ID, transaction, and best-effort persistence semantics for SQLite and Postgres.
- [x] Decide configuration and failure behavior: default SQLite, explicit Postgres selection, no silent fallback, and actionable startup errors.
- [x] Add a contract-test strategy that both implementations must satisfy, including room isolation and retention behavior.
- [x] Record the chosen Go driver/pool approach, connection limits, TLS/secret handling, and migration ownership with links to primary docs.

## Context
SQLite currently lives in `backend/db/database.go` and owns migrations, retention, history, message IDs, and moderation audit persistence. Keep the public HTTP/WebSocket behavior stable while extracting only a real boundary.

## Notes
Research anchors: https://www.postgresql.org/docs/current/libpq-connect.html, https://www.postgresql.org/docs/current/transaction-iso.html, and https://github.com/jackc/pgx. Do not add Postgres dependency churn until this contract is reviewed.

## Verification

- Added `docs/adr/0008-storage-repository-boundary.md` and indexed it in `docs/adr/README.md`.
- Added deterministic `(timestamp, id)` history ordering and UTC persistence timestamps in the SQLite adapter.
- `go test -race ./db` passes, including `TestGetRecentMessagesUsesIDForTimestampTies`.
