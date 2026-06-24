# 5. Buffered broadcast channel and per-client write lock

## Status

Accepted

## Context

The hub delivered history from a **goroutine** while the main hub loop **broadcast** join notifications. Both called `WriteJSON` on the same `*websocket.Conn`. The gorilla WebSocket implementation allows **only one concurrent writer** per connection.

Separately, an **unbuffered** `broadcast` channel caused a **deadlock**: the client’s `readMessages` loop blocked sending to `broadcast` while the hub tried to `WriteJSON` to that same client.

## Decision

1. Use a **buffered** `broadcast` channel (capacity 1024) so producers do not block on the hub consuming the next message.
2. Serialize all server-origin writes through **`Client.writeJSON`** guarded by a **mutex** per client.

## Consequences

**Positive:** No writer races; no self-deadlock on single-client broadcast paths.

**Negative:** Mutex adds latency under extreme fan-out; superseded in part by [ADR 0006](0006-bounded-client-outbound-queues.md), which adds one outbound goroutine per client with a bounded channel of outbound frames.
