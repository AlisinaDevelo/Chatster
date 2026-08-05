# 2. In-process WebSocket hub

## Status

Accepted

## Context

Real-time delivery must be low-latency for a classroom-scale demo. The team wants to avoid operating Redis or NATS for the default experience.

## Decision

Maintain one hub event loop (`Hub.run`) for broadcast and unregister work, plus a
mutex-protected admission path for registration and shutdown state. The hub keeps an
in-memory `map` of clients and uses no external broker for the default experience.

## Consequences

**Positive:** Simple mental model; easy to read; no extra infrastructure.

**Negative:** **Not horizontally scalable** without a shared pub/sub layer; the broadcast loop can become a bottleneck; the hub still keeps all live connection state in one process.

**Follow-up:** Introduce Redis Pub/Sub (or equivalent) and sticky sessions—or a dedicated gateway—when running multiple replicas. Shutdown draining is defined in [ADR 0007](0007-websocket-drain-lifecycle.md).
