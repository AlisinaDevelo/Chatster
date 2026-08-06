# 8. Storage repository boundary for Postgres mode

## Status

Accepted

## Context

Chatster currently passes `*db.DB` through the HTTP and WebSocket layers. The SQLite
wrapper owns message IDs, room history, moderation audit events, retention, health checks,
and startup migrations. That is appropriate for the default single-node deployment, but it
would make a Postgres mode a risky rewrite if handlers learn SQL or backend-specific failure
rules.

The next storage implementation must preserve the public API and WebSocket behavior. It must
also make the choice of storage explicit: a failed Postgres connection must never silently
fall back to a new empty SQLite database.

## Decision

The application will depend on a narrow repository contract. Concrete adapters own SQL,
connection setup, migrations, and driver-specific error translation. Handlers, the hub, and
retention startup code do not receive `*sql.DB`.

The contract is implemented in the `backend/db` package. The interface below is a compact
description of the operations and semantics; the concrete Go names also preserve compatibility
helpers for the existing SQLite tests:

```go
type Repository interface {
	MessageStore
	ModerationStore
	HealthStore
	RetentionStore

	Close() error
}

type MessageStore interface {
	SaveMessage(ctx context.Context, input MessageInput) (Message, error)
	RecentMessages(ctx context.Context, room string, limit int) ([]Message, error)
}

type ModerationStore interface {
	SaveModerationEvent(ctx context.Context, input AuditEventInput) (ModerationEvent, error)
}

type HealthStore interface {
	Ping(ctx context.Context) error
}

type RetentionStore interface {
	PruneMessagesBefore(ctx context.Context, cutoff time.Time) (int64, error)
	PruneModerationEventsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type Migrator interface {
	Migrate(ctx context.Context) error
}

type StartupError struct {
	Backend   string
	Operation string
	Err       error
}
```

`OpenRepository(ctx, config)` is the only application construction path. It selects exactly one adapter, runs or
verifies its migrations, pings the backend, and returns a wrapped `StartupError` on failure.
The error names the backend and operation, but never includes a password, full DSN, or secret
environment value.

### Behavioral contract

| Concern | Required behavior |
| --- | --- |
| Room input | The application canonicalizes room names before calling the repository. Adapters validate defensively and must never return messages from another room. |
| History | `RecentMessages` returns at most `limit` messages, oldest to newest. Selection and ordering use `(timestamp, id)`; the ID breaks ties deterministically. Empty history is an empty collection, not an error. |
| Timestamp | The adapter assigns the persistence timestamp in UTC and returns the stored value. Client-provided timestamps are not accepted. |
| ID | A persisted message receives a positive, backend-local identity after insert. IDs are never reused; gaps are allowed. Message IDs are not assumed to be globally unique across storage backends or instances. |
| Message transaction | An accepted durable message is committed before it is broadcast. A failed save is not broadcast. A timeout after an uncertain commit must not be retried automatically without an idempotency key, because the row may already exist. |
| System notifications | The existing `username` handshake notification is not durable and never appears in history; it keeps an ID of zero. Durable chat message types follow the normal save contract. |
| Audit events | Rejection auditing is best effort. A failed audit write is logged and counted, but it cannot turn a rejected input into an accepted message or change the user-visible rejection. Audit content remains bounded by the current preview and length rules. |
| Retention | Pruning deletes rows strictly older than the UTC cutoff, returns the affected-row count, and is atomic per table. Message and audit retention are independent operations. |
| Health | `Ping(ctx)` honors cancellation and deadlines. `/health` reports degraded when the bounded ping fails. Startup open, migration, and initial ping failures are fatal and actionable. |
| Concurrency | SQLite preserves its one-connection/write-serialization behavior. Postgres uses normal `READ COMMITTED` transactions; concurrent writers have no cross-request ordering promise beyond the returned `(timestamp, id)` history order. |

### Configuration and failure behavior

The implementation uses explicit storage selection:

| Variable | Contract |
| --- | --- |
| `CHATSTER_STORAGE` | `sqlite` by default; `postgres` opts into the Postgres adapter. Unknown values fail startup. |
| `CHATSTER_DB_PATH` | SQLite database path, used only when storage is `sqlite`. |
| `CHATSTER_POSTGRES_DSN` | Secret connection string, required when storage is `postgres`; it is never logged. |
| `CHATSTER_POSTGRES_MIN_CONNS` | Postgres pool floor, default `2`. |
| `CHATSTER_POSTGRES_MAX_CONNS` | Postgres pool ceiling, default `10`; must be positive and at least the minimum. |

Selecting `postgres` without a valid DSN, TLS configuration, reachable server, or compatible
schema exits with a startup error. Selecting `sqlite` never attempts Postgres. There is no
automatic fallback in either direction, and the default Render deployment remains SQLite
until a separately verified Postgres mode is chosen.

For Postgres, the selected driver is `github.com/jackc/pgx/v5` with `pgxpool`. The initial
pool defaults are deliberately small for Chatster's workload and are configurable without
creating a pool per request. Every acquire and query receives the caller's context. The
implementation must validate pool bounds and expose pool health without exposing credentials.

Production Postgres connections require TLS with hostname verification (`sslmode=verify-full`
or the equivalent pgx TLS configuration) and a trusted CA. A weaker mode is an explicit local
development choice only. Credentials belong in the deployment secret manager or environment,
not in `render.yaml`, logs, test fixtures, or committed `.env` files.

SQLite keeps its current ordered startup migrations and one-connection configuration. Postgres
migrations are application-owned, versioned, forward-only, and must produce the same logical
schema and retention behavior. A Postgres migration run acquires a database advisory lock so
only one replica migrates; all replicas fail startup if migration or schema verification fails.
Migration ownership remains with the Chatster release rather than an operator running ad hoc
SQL against production.

### Contract-test strategy

The `db` package uses a test-only factory shape that both adapters satisfy:

```go
type repositoryFactory func(t *testing.T) db.Repository

func TestRepositoryContract(t *testing.T, newRepository repositoryFactory) {
	// Run the same cases for SQLite and Postgres.
}
```

The shared suite must cover:

1. General/engineering room isolation, canonical room names, and empty history.
2. Chronological history with equal timestamps, using message ID as the tie-breaker.
3. Positive non-reused IDs, server timestamps, durable messages, and non-durable username handshakes.
4. Audit preview truncation, Unicode length accounting, and best-effort error handling.
5. Message and audit retention around the exact cutoff, including returned delete counts.
6. Context cancellation for health/history and idempotent migration startup.
7. Invalid storage selection, missing Postgres DSN, migration failure, and ping failure as actionable startup errors with no fallback.

The SQLite suite runs on every normal test invocation. The Postgres suite is enabled by a
dedicated CI service or `CHATSTER_POSTGRES_TEST_DSN` and must never make the default SQLite
job depend on external infrastructure. The tests compare observable contract behavior, not
driver-specific SQL plans.

## Consequences

**Positive:** The Postgres adapter sits behind a stable boundary; storage failures are explicit;
room isolation, history ordering, audit semantics, and retention cannot drift between backends
without failing the shared contract suite.

**Negative:** The repository boundary adds a small translation layer and requires two migration
sets. Postgres pooling, TLS, backups, and migration coordination become operational concerns
when that mode is enabled.

**Implementation:** Issue [#25](https://github.com/AlisinaDevelo/Chatster/issues/25) adds the
optional Postgres adapter, context-aware repository boundary, shared contract tests, and the
independent CI service job described here.

## References

- [PostgreSQL connection control and `sslmode`](https://www.postgresql.org/docs/current/libpq-connect.html)
- [PostgreSQL transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html)
- [pgx PostgreSQL driver](https://github.com/jackc/pgx)
- [pgxpool package documentation](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool)
