# 4. Prometheus metrics on `/metrics`

## Status

Accepted

## Context

Reviewers expect **observability hooks** beyond stdout logs. OpenTelemetry is powerful but heavier to configure for a small service.

## Decision

Expose **Prometheus** exposition format at **`GET /metrics`** using `prometheus/client_golang`, with a small set of **Chatster-specific** counters/gauges alongside default Go runtime metrics.

## Consequences

**Positive:** Works with Grafana, Prometheus, and managed monitoring; low config for demos.

**Negative:** No automatic trace correlation; histograms/SLO burn rates need additional instrumentation later.

**Follow-up:** Add histograms for DB and broadcast latency; adopt OTel for traces when deploying to a mesh or hosted APM.
