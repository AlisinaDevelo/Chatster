# 7. WebSocket drain lifecycle

## Status

Accepted

## Context

`net/http.Server.Shutdown` stops new HTTP work but does not own hijacked WebSocket
connections. Leaving the in-memory hub alive during process termination can strand client
read/write goroutines and makes deploy behavior dependent on the runtime's final socket
cleanup.

## Decision

The hub has an explicit draining lifecycle. Shutdown first flips admission to draining,
which rejects new WebSocket upgrades with HTTP 503. Existing clients receive WebSocket
close code **1012 (service restart)** and the application closes their connections. The
hub waits for each client read loop until the shared 30-second shutdown context expires,
then records any remaining clients as forced closes before stopping the hub goroutine.

Drain start, duration, remaining clients, and forced closes are exposed through structured
logs and Prometheus metrics. Logs contain lifecycle counts only and never message bodies.

## Consequences

**Positive:** Deploys have a bounded, observable connection lifecycle; clients get a
standard restart signal and can reconnect; shutdown tests can verify hub and client
completion independently of the operating-system signal path.

**Negative:** Clients reconnect during a rolling restart, and a forced-close count means
some client loops outlived the shutdown budget. The reference deployment remains a
single-node in-memory hub.
