# 6. Bounded per-client outbound queues

## Status

Accepted

## Context

The original hub loop wrote broadcast messages directly to every client socket. Even with a per-client write mutex, one slow client could hold the hub in a socket write and delay fan-out to every other connected client.

The server also needs a deliberate backpressure policy. An unbounded queue can hide a slow client until memory grows; blocking the hub can punish healthy clients.

## Decision

Each WebSocket client owns a bounded outbound channel and one writer goroutine. The hub enqueues broadcast messages into each client's queue instead of writing directly to the socket.

If a client's outbound queue is full, the server treats it as a slow-client failure:

- increment `chatster_websocket_outbound_drops_total{reason="slow_client"}`;
- close the client connection;
- remove it from the active hub client set.

Write failures in the client writer increment `chatster_websocket_outbound_drops_total{reason="write_error"}` and close the connection.

## Consequences

**Positive:** A slow socket no longer blocks hub fan-out to healthy clients. Backpressure is explicit, bounded, and observable.

**Negative:** A client can be disconnected during a short network stall if its outbound queue fills. The queue size is intentionally conservative for the demo and should be tuned with production traffic.

**Follow-ups:** Expose queue depth if this becomes a real service, and consider per-room fan-out or pub/sub when the single-process hub becomes the bottleneck.
