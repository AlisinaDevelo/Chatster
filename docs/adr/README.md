# Architecture Decision Records

Short records of **why** Chatster is shaped the way it is. New significant decisions should add `NNNN-title.md` here.

| ADR | Title |
|-----|--------|
| [0001](0001-sqlite-single-node.md) | SQLite for single-node persistence |
| [0002](0002-in-memory-websocket-hub.md) | In-process WebSocket hub |
| [0003](0003-no-auth-demo-scope.md) | Anonymous authentication remains the demo default (superseded in part by 0010) |
| [0004](0004-prometheus-metrics.md) | Prometheus metrics on `/metrics` |
| [0005](0005-broadcast-channel-and-writer-lock.md) | Buffered broadcast + per-client WS write lock |
| [0006](0006-bounded-client-outbound-queues.md) | Bounded per-client outbound queues |
| [0007](0007-websocket-drain-lifecycle.md) | Explicit WebSocket drain lifecycle on shutdown |
| [0008](0008-storage-repository-boundary.md) | Storage repository boundary for Postgres mode |
| [0009](0009-cross-instance-room-event-contract.md) | Cross-instance room event contract |
| [0010](0010-opt-in-signed-session-auth.md) | Opt-in signed session authentication |

## Template

```markdown
# N. Title

## Status

Accepted | Proposed | Superseded by ADR X

## Context

## Decision

## Consequences

Positive / negative / follow-ups
```
