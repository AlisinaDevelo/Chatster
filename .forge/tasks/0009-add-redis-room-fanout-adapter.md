---
id: 0009
title: Add an optional Redis room fan-out adapter
status: done
agent: backend-specialist
model: sonnet
depends_on: [0008]
parent: 0001
---

## Goal
Allow multiple Chatster instances to deliver live room messages through an optional Redis adapter while retaining local persistence and HTTP history as the source of truth.

## Acceptance criteria
- [x] Broker mode is disabled by default and fails clearly when enabled without a valid Redis connection/configuration.
- [x] Publish and subscribe channels are namespaced by environment and room, with loop prevention and bounded broker-to-client backpressure.
- [x] A Docker-based integration test starts two Chatster instances and Redis, proving same-room delivery, cross-room isolation, duplicate handling, and reconnect history.
- [x] Metrics/logs expose broker connection, publish, subscribe, decode, drop, and reconnect outcomes without message-body leakage.
- [x] Documentation states Redis Pub/Sub at-most-once semantics and names Redis Streams or another durable path as the follow-up when replay/at-least-once delivery is required.

## Context
Dependency 0008 is complete. Preserve the current SQLite-only and in-process modes, and keep room authorization/authentication separate from transport fan-out.

## Notes
Research anchor: https://redis.io/docs/latest/develop/pubsub/. Do not claim durable delivery from Pub/Sub alone.

## Verification

- Local: `CHATSTER_REDIS_TEST_URL=redis://127.0.0.1:6379/15 go test -race ./...`
- Local: `go vet ./...`, frontend lint, unit tests, production build, and Chromium E2E all passed.
- CI: [run 31233953390](https://github.com/AlisinaDevelo/Chatster/actions/runs/31233953390) passed backend, frontend, Postgres, and production-image jobs.
