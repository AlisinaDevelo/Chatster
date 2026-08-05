---
id: 0004
title: Gracefully drain WebSocket clients on shutdown
status: done
agent: backend-specialist
model: sonnet
depends_on: []
parent: 0001
---

## Goal
Make deploys and process termination predictable by explicitly draining active WebSocket clients instead of relying only on HTTP shutdown behavior.

## Acceptance criteria
- [ ] Shutdown stops or rejects new WebSocket registrations before the drain deadline begins.
- [ ] Active clients receive a documented close signal/code and are closed within a bounded timeout, including clients blocked in normal read/write loops.
- [ ] Shutdown does not leak hub/client goroutines; add an integration test that exercises signal or an equivalent server shutdown hook.
- [ ] Logs and Prometheus metrics expose drain start, duration, remaining clients, and forced-close behavior without logging message bodies.
- [ ] `docs/ARCHITECTURE.md`, `docs/OPERATIONS.md`, and the relevant ADR describe the lifecycle and operator expectations.

## Context
Current code has graceful HTTP shutdown, heartbeat, bounded outbound queues, and reconnecting clients, but the in-memory hub has no explicit drain lifecycle. Preserve normal room fan-out and the existing 30-second HTTP shutdown budget unless tests show a better bounded contract.

## Notes
Reference: https://pkg.go.dev/net/http#Server.Shutdown. WebSocket connections are long-lived and need an explicit application-level lifecycle around HTTP shutdown.

## Verification
- `go test -race ./...` and focused shutdown/WebSocket race tests (five repetitions) pass.
- `go vet ./...` and `golangci-lint` v2.12.2 report no issues; frontend tests/build and CI YAML validation pass.
- GitHub Actions run 30973338168 passed backend, frontend, and production-image jobs.

## Verification
- `go test -race ./...` passes, including real WebSocket close-code/rejection coverage and the forced-deadline branch.
- Focused `TestHubShutdown|TestWebSocket` race tests pass across five repetitions.
- `go vet ./...`, `golangci-lint` v2.12.2, frontend tests/build, and CI YAML validation pass locally.
