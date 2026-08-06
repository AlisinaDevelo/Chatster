---
id: 0007
title: Add an optional Postgres repository mode
status: done
agent: backend-specialist
model: sonnet
depends_on: [0006]
parent: 0001
---

## Goal
Add a production-oriented Postgres storage mode behind the repository boundary while preserving SQLite as the zero-dependency default for local development and the public single-node demo.

## Acceptance criteria
- [x] `CHATSTER_STORAGE=sqlite|postgres` (or the reviewed equivalent) selects the backend explicitly and fails fast on missing/invalid Postgres configuration.
- [x] Postgres migrations cover messages, rooms, moderation audit, retention indexes, and any new schema versions without weakening SQLite migration behavior.
- [x] Contract tests prove room filtering, timestamp/ID ordering, retention, health checks, and audit persistence against both backends.
- [x] CI runs a real Postgres service/integration job without making the SQLite path depend on external infrastructure.
- [x] Deployment and operations docs cover pooling, TLS, secrets, migrations, backups, and the single-node SQLite versus Postgres tradeoff.

## Context
The storage boundary is implemented by ADR 0008. The default Render Blueprint remains SQLite;
Postgres is opt-in until an operator supplies a managed database and verifies its operational
requirements.

## Notes
Use the primary PostgreSQL connection and transaction documentation linked from issue 0006. Keep accepted message ordering and room isolation identical across storage modes.
