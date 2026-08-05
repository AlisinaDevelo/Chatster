---
id: 0005
title: Add opt-in OpenTelemetry tracing
status: backlog
agent: observability-specialist
model: sonnet
depends_on: []
parent: 0001
---

## Goal
Add a small, opt-in distributed tracing path that connects HTTP requests, WebSocket upgrades, message persistence, and room fan-out without changing the default demo footprint or exposing message content.

## Acceptance criteria
- [ ] Initialize an OTel `TracerProvider` only when configured, with `service.name=chatster` and clean shutdown flushing.
- [ ] Trace HTTP health/history requests, WebSocket upgrade/connection lifecycle, message persistence, and broadcast enqueue work with useful room-safe attributes.
- [ ] Use standard OTel configuration where possible (`OTEL_SERVICE_NAME`, OTLP endpoint/exporter settings); the default local and CI path remains exporter-free.
- [ ] Add tests with an in-memory exporter or equivalent that verify spans exist and message bodies/secrets are absent.
- [ ] Document collector setup, sampling, shutdown behavior, and the relationship between traces, Prometheus metrics, and `slog` logs.

## Context
`docs/OBSERVABILITY.md` already describes Prometheus metrics, structured logs, and an OTel adoption outline. Avoid replacing stable metrics; add traces as a separate signal and keep high-cardinality room/user data bounded or redacted.

## Notes
Research anchors: https://opentelemetry.io/docs/languages/go/ and https://opentelemetry.io/docs/languages/go/instrumentation/. OTel Go documents traces as stable and recommends combining library instrumentation with custom application spans.
