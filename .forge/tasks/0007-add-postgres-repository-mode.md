---
id: 0007
title: Add an optional Postgres repository mode
status: in-progress
agent: backend-specialist
model: sonnet
depends_on: [0006]
parent: 0001
---

## Goal
Add a production-oriented Postgres storage mode behind the repository boundary while preserving SQLite as the zero-dependency default for local development and the public single-node demo.

## Acceptance criteria
- [ ] `CHATSTER_STORAGE=sqlite|postgres` (or the reviewed equivalent) selects the backend explicitly and fails fast on missing/invalid Postgres configuration.
- [ ] Postgres migrations cover messages, rooms, moderation audit, retention indexes, and any new schema versions without weakening SQLite migration behavior.
- [ ] Contract tests prove room filtering, timestamp/ID ordering, retention, health checks, and audit persistence against both backends.
- [ ] CI runs a real Postgres service/integration job without making the SQLite path depend on external infrastructure.
- [ ] Deployment and operations docs cover pooling, TLS, secrets, migrations, backups, and the single-node SQLite versus Postgres tradeoff.

## Context
Blocked by the storage boundary/design issue. Do not change the default Render Blueprint until the Postgres mode is proven and its operational cost is documented.

## Notes
Use the primary PostgreSQL connection and transaction documentation linked from issue 0006. Keep accepted message ordering and room isolation identical across storage modes.
