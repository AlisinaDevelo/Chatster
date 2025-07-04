# 1. SQLite for single-node persistence

## Status

Accepted

## Context

The demo needs durable chat history with minimal operational overhead. Operators may run a single container or binary on one host.

## Decision

Use **SQLite** with a single file (`CHATSTER_DB_PATH`), accessed through `database/sql` with the `go-sqlite3` driver (CGO).

## Consequences

**Positive:** No separate database server; fast local dev; simple backups (copy file); fits Docker volume mounts.

**Negative:** Single-writer semantics; poor fit for multi-instance write scaling; file locking under heavy concurrency.

**Follow-up:** Move to **Postgres** (or similar) when requiring horizontal write scaling or HA.
