# Scaling and failure modes

Chatster is intentionally a **single-process, single-node** reference. This document states **what breaks first**, **symptoms**, and **typical next steps** if you outgrow the demo—without requiring those changes in this repository.

## Current architecture constraints

| Constraint | Implication |
|------------|-------------|
| One Go process | All WebSocket fan-out and HTTP share CPU, memory, and goroutine budget. |
| In-memory hub | Connected clients and room membership remain process-local; optional Redis Pub/Sub bridges live room events between instances. |
| SQLite (single file) | One writer at a time; concurrent writes queue; file on local disk. |
| Synchronous broadcast loop | Slow `WriteJSON` to one client blocks the hub from processing the next broadcast until that iteration completes (mitigated partially by closing dead clients). |
| Unbuffered fan-out | Sending on an **unbuffered** `broadcast` channel from the same connection’s read loop **deadlocks** with the hub writing to that socket; this repo uses a **buffered** channel and **per-connection write locks** (see `Client.writeJSON`). |

## What breaks first (order of pain)

1. **WebSocket fan-out under load** — Broadcasting to thousands of clients from one goroutine loop increases latency per message; slow clients stall others. **Symptoms:** rising end-to-end message delay, timeouts, growing goroutines if work leaks.  
   **Next steps:** shard the existing room-scoped hub by room; move to a **message bus** (Redis Pub/Sub, NATS, Kafka) so instances publish once; or use a dedicated real-time layer (managed WebSocket/Pusher-class service).

2. **SQLite write throughput** — Every persisted message serializes on the DB file. **Symptoms:** `database is locked`, growing save latency, `/health` degrading under ping+write contention.  
   **Next steps:** use the implemented **Postgres** repository mode with connection pooling; then consider batch writes or separate read replicas for history if reads dominate.

3. **Horizontal scaling of the API** — Multiple instances behind a load balancer each hold **different** in-memory client sets. **Symptoms:** users on instance A never see broadcasts from instance B.  
   **Next steps:** use **sticky sessions** (same instance for WS) plus the implemented Redis fan-out, or move to a single shared connection gateway.

4. **Backpressure** — Malicious or buggy clients can send large or frequent frames. **Symptoms:** memory pressure, JSON decode cost, hub channel buildup if producers outpace consumers.  
   **Next steps:** stricter **frame size limits**, **per-IP rate limits**, **queue depth caps** with drop policies, **circuit breakers**.

5. **Operational blast radius** — Process crash drops all connections and in-flight hub state (SQLite survives on disk). **Symptoms:** mass disconnect; clients should reconnect (your client already retries).  
   **Next steps:** multiple replicas, health-based routing, shorter DNS TTLs, connection draining on shutdown (extend hub shutdown beyond HTTP `Shutdown`).

## Non-goals for this repo

- Multi-region active-active chat.
- Guaranteed ordering across shards.
- Infinite history at line-rate ingest.

See [adr/README.md](adr/README.md) for decisions that keep the stack small.

The reviewed scale-out contracts are [ADR 0008](adr/0008-storage-repository-boundary.md) for
durable storage and [ADR 0009](adr/0009-cross-instance-room-event-contract.md) for live room
fan-out. Postgres storage and Redis/event fan-out are implemented and remain opt-in; both are
needed for a multi-instance deployment.

## Redis fan-out (opt-in)

Set `CHATSTER_REDIS_URL` to enable cross-instance live room broadcasts. Configure the same
namespace for every instance in one environment and give each process a unique instance ID:

```text
CHATSTER_STORAGE=postgres
CHATSTER_POSTGRES_DSN=postgres://chatster:<secret>@<host>:5432/chatster?sslmode=verify-full
CHATSTER_REDIS_URL=rediss://:<secret>@<host>:6379/0
CHATSTER_REDIS_NAMESPACE=production
CHATSTER_INSTANCE_ID=chatster-web-1
```

Each accepted, persisted message is published to a versioned room channel. Subscribers validate
the envelope, ignore their own origin, deduplicate a bounded recent-event window, and deliver
remote events through the existing bounded WebSocket queues. Redis failure never replaces the
durable write or history API. The `chatster_redis_*` metrics expose connection, publish,
subscription, decode, drop, and reconnect outcomes without recording message bodies.

Redis Pub/Sub is an at-most-once live transport: a subscriber that is disconnected can miss an
event. Reconnects should use the shared history endpoint to catch up. Redis Streams remain a
follow-up for stronger delivery guarantees. CI runs the two-instance integration against a
Docker Redis service; locally, set `CHATSTER_REDIS_TEST_URL` before running the backend tests.
