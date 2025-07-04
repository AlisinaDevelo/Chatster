# 2. In-process WebSocket hub

## Status

Accepted

## Context

Real-time delivery must be low-latency for a classroom-scale demo. The team wants to avoid operating Redis or NATS for the default experience.

## Decision

Maintain a **single goroutine** (`Hub.run`) with channels for register/unregister/broadcast and an in-memory `map` of clients.

## Consequences

**Positive:** Simple mental model; easy to read; no extra infrastructure.

**Negative:** **Not horizontally scalable** without a shared pub/sub layer; broadcast loop can become a bottleneck; graceful drain of WS on shutdown is not fully implemented.

**Follow-up:** Introduce Redis Pub/Sub (or equivalent) and sticky sessions—or a dedicated gateway—when running multiple replicas.
