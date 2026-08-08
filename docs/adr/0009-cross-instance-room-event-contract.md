# 9. Cross-instance room event contract

## Status

Accepted

## Context

The current hub is intentionally in-process. A future multi-instance deployment needs a
transport event that can be validated, scoped to one room, and delivered without making the
broker a second source of truth. The storage repository remains authoritative for accepted
messages and the HTTP history endpoint remains authoritative after reconnect.

## Decision

The future fan-out adapter uses a versioned JSON envelope. The executable shape and validation
rules live in [`backend/internal/events`](../../backend/internal/events); the fixture at
[`room-message-v1.json`](../../backend/internal/events/testdata/room-message-v1.json) is the
compatibility example.

```json
{
  "schema": "chatster.room-message",
  "version": 1,
  "event_id": "01JH7Q4ZK5N2R8S6T1V3W9X0Y4",
  "room": "general",
  "origin_instance": "instance-a",
  "timestamp": "2026-08-06T10:00:00Z",
  "message": {
    "id": 42,
    "user_id": "usr_alice",
    "username": "alice",
    "content": "hello from the room",
    "type": "message"
  }
}
```

### Envelope fields

| Field | Contract |
| --- | --- |
| `schema` | Exact marker `chatster.room-message`; unknown schemas are rejected and counted as decode failures. |
| `version` | Integer `1`; a future incompatible version gets a separate decode result and is not guessed at. |
| `event_id` | Globally unique opaque ID, generated once after persistence and reused for bounded retries. It is the deduplication key; a message ID alone is not sufficient across instances. |
| `room` | Canonical Chatster room name. It is duplicated in the channel path and validated before delivery. |
| `origin_instance` | Stable, log-safe instance ID for the process lifetime. It is configured with `CHATSTER_INSTANCE_ID` or generated at startup; duplicate configured IDs are an operator error. |
| `timestamp` | UTC persistence timestamp from the repository, not a subscriber or broker receive time. |
| `message` | Durable message payload: positive storage ID, optional stable `user_id`, username, content, and message type. The additive user ID preserves authenticated ownership across instances; anonymous and system rows omit it. Handshake messages are never broker events. |

### Channel and delivery rules

Channels use the exact form:

```text
chatster:v1:<namespace>:room:<canonical-room>
```

`<namespace>` is a configured lowercase token, such as `staging` or `production`, and may
contain only letters, digits, hyphens, and underscores. The namespace must be distinct per
environment so a staging publisher cannot reach production subscribers. Subscribers use exact
room channels; pattern subscriptions are not part of the contract.

On the origin instance, the sequence is:

1. Persist the accepted message in the shared repository.
2. Create one envelope with one event ID.
3. Deliver it to the local room hub.
4. Publish the same bytes to the room channel.

A subscriber validates the schema, version, room, origin, and durable message fields before
passing the event to its local hub. It does not persist, republish, or reassign the message
ID. The origin instance drops its own broker copy because it already delivered locally.
Subscribers keep a bounded, TTL-limited event ID set to suppress duplicate deliveries and
never emit a second broker event for a duplicate.

Live ordering is deliberately limited. Redis delivers messages on one subscription in publish
order, but concurrent origins do not receive a cross-instance total-order guarantee. The
envelope timestamp and storage ID describe canonical history order; live consumers may receive
events in arrival order and must reconcile through the history API on reconnect. There is no
ordering guarantee across rooms.

### Reconnects and broker failures

`GET /api/messages?room=<room>&limit=<n>` remains the replay authority. Redis Pub/Sub events
are not replayed after a disconnect. When a WebSocket reconnects, the client fetches history
and merges the canonical result before relying on new live events.

If the broker is unavailable after local persistence, the sender still receives the accepted
message and local clients still see it. The publisher records a failure, logs only bounded
metadata, and may retry the same envelope with the same event ID using a bounded backoff. It
must not create a new event ID for an uncertain publish result. Remote instances may miss live
delivery during the outage; their clients catch up from durable history after reconnect. A
broker outage must not cause an accepted message to be written to a second local database or
silently downgrade storage.

### Pub/Sub versus Streams

The first optional adapter will use **Redis Pub/Sub** for live fan-out only. It is sufficient
because Chatster already persists accepted messages and replays them from the repository; the
broker does not need to provide history or acknowledgements. Pub/Sub is at-most-once: a
disconnected subscriber loses the event, which is an explicit and observable tradeoff.

**Redis Streams** is the follow-up transport when the product requires broker-side replay,
consumer groups, acknowledgements, at-least-once processing, or recovery of events missed during
an outage. Streams add retention, pending-entry recovery, consumer ownership, and operational
policy. Switching to Streams is a separate adapter decision and does not change the envelope
schema or make the repository optional.

### Metrics

The Redis adapter will expose bounded-label Prometheus metrics:

| Metric | Labels / meaning |
| --- | --- |
| `chatster_broker_publish_total{result}` | Publish attempts and outcomes (`ok`, `error`, `timeout`). |
| `chatster_broker_subscribe_total{result}` | Subscription and reconnect outcomes. |
| `chatster_broker_decode_total{result}` | Envelope decode/validation outcomes (`ok`, `invalid_json`, `unsupported_schema`, `invalid_payload`). |
| `chatster_broker_duplicates_total` | Events suppressed by event ID deduplication. |
| `chatster_broker_failures_total{operation}` | Broker failures by bounded operation (`connect`, `publish`, `receive`, `close`). |
| `chatster_broker_reconnects_total{result}` | Subscriber reconnect attempts and outcomes. |

Room names, event IDs, instance IDs, usernames, and message bodies are never metric labels.
Structured logs use the same bounded metadata and never include message content.

### Adapter contract tests

The `internal/events` package tests the fixture, schema/version rejection, timestamp
normalization, canonical room/channel construction, and durable-message validation. The future
Redis integration fixture should run two Chatster instances and a Redis service through the
same cases:

1. Same-room delivery reaches clients on both instances exactly once.
2. Different rooms never cross channels.
3. The origin instance does not rebroadcast its own broker copy.
4. Repeated delivery of one event ID is suppressed without republishing.
5. A broker disconnect does not lose the persisted row; reconnect plus history restores it.
6. Invalid JSON, schema, version, room, and payload are dropped and counted.
7. A publish timeout keeps the original event ID and does not create a duplicate row.

These tests are intentionally not part of the default runtime until the Redis adapter task is
implemented.

## Consequences

**Positive:** Every future transport has one testable wire format, room scoping is explicit,
and broker failure cannot replace durable history. Pub/Sub keeps the first scale-out step small.

**Negative:** Pub/Sub can lose live events and provides no replay or acknowledgement. The
bounded dedupe cache is process-local, and global live ordering is not promised.

**Follow-up:** Issue [#27](https://github.com/AlisinaDevelo/Chatster/issues/27) implements the
optional Redis adapter after the storage mode and this event contract are reviewed.

## References

- [Redis Pub/Sub](https://redis.io/docs/latest/develop/pubsub/)
- [Redis Streams](https://redis.io/docs/latest/develop/data-types/streams/)
