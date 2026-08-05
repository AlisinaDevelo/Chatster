---
id: 0008
title: Define the cross-instance room event contract
status: ready
agent: architect
model: opus
depends_on: []
parent: 0001
---

## Goal
Define the event envelope and delivery assumptions needed before Chatster can broadcast room messages across multiple Go instances.

## Acceptance criteria
- [ ] Add an ADR for event fields: event ID, room, origin instance, message payload, timestamp, and schema/version marker.
- [ ] Specify ordering, duplicate handling, loop prevention, reconnect/history authority, and behavior when the broker is unavailable.
- [ ] Decide whether Redis Pub/Sub is sufficient for live fan-out or whether Streams are required for replay/stronger delivery; document the tradeoff explicitly.
- [ ] Define namespaced channel conventions and metrics for publish, subscribe, decode, duplicate, and broker failures.
- [ ] Add a test-fixture or contract outline that can be reused by a future Redis adapter.

## Context
The current hub is room-scoped but in-process only. Accepted messages are persisted in the local database and history replay is authoritative on reconnect; the broker should not become a second source of truth.

## Notes
Research anchors: https://redis.io/docs/latest/develop/pubsub/ and https://redis.io/docs/latest/develop/data-types/streams/. Redis documents Pub/Sub as at-most-once and recommends Streams when persistence, replay, or stronger delivery is required.
