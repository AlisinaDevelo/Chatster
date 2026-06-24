# Architecture Decision Records

Short records of **why** Chatster is shaped the way it is. New significant decisions should add `NNNN-title.md` here.

| ADR | Title |
|-----|--------|
| [0001](0001-sqlite-single-node.md) | SQLite for single-node persistence |
| [0002](0002-in-memory-websocket-hub.md) | In-process WebSocket hub |
| [0003](0003-no-auth-demo-scope.md) | No authentication in the reference scope |
| [0004](0004-prometheus-metrics.md) | Prometheus metrics on `/metrics` |
| [0005](0005-broadcast-channel-and-writer-lock.md) | Buffered broadcast + per-client WS write lock |
| [0006](0006-bounded-client-outbound-queues.md) | Bounded per-client outbound queues |

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
