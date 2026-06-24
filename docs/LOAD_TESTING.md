# Load Testing

Chatster includes a small Go WebSocket load generator so benchmark claims can be reproduced without extra tooling. It measures **end-to-end broadcast delivery latency** (send → delivered to a connected client) and **delivered throughput**, not just how fast the client can enqueue sends.

## Run

Start the backend (limiters disabled here to measure server capacity rather than the demo-safe defaults):

```bash
cd backend
CHATSTER_MESSAGE_RPS=0 CHATSTER_WS_UPGRADE_RPS=0 go run .
```

In another terminal:

```bash
cd backend
go run ./cmd/wsload \
  -url ws://127.0.0.1:8080/ws \
  -clients 25 \
  -messages 20 \
  -settle-for 500ms \
  -send-interval 5ms \
  -drain-for 3s \
  -fail-on-loss
```

The command prints a JSON summary. Key fields:

- `delivered` — broadcast frames that carried a load message and were received by a connected client (each message fans out to every connected client).
- `expected_delivered` / `lost` — expected fan-out deliveries (`total_sent × clients`) and the gap between expected and measured deliveries.
- `delivered_throughput_per_second` — `delivered` over the window from the first send to the last in-scope delivery.
- `latency_p50_ms` / `p95` / `p99` / `max` / `mean` — round-trip latency from send to delivery, across every delivery.
- `total_sent` / `total_received` — attempted sends and all inbound frames (including history replay, join/leave, and server notices).
- `settle_for` — startup grace period after username setup; this keeps connection/bootstrap messages out of the timed send window.
- `send_interval` — per-client pacing between messages; use a non-zero value for a sustained run instead of a single microburst.

## Results

Measured on an **Apple M1 (8 cores), macOS 26.5, Go 1.26.3**, 2026-06-24, with both rate limiters disabled and a **fresh SQLite database per run**. Each client sent 20 messages paced at 5 ms; every message is broadcast to all connected clients, so deliveries scale as O(clients²).

| Clients | Delivered | Lost | Throughput (deliveries/s) | p50 | p95 | p99 |
|--------:|----------:|-----:|--------------------------:|----:|----:|----:|
| 25  | 12,500  | 0 | ~122,000 | 2.3 ms | 4.7 ms | 5.6 ms |
| 50  | 50,000  | 0 | ~224,000 | 71 ms | 119 ms | 124 ms |
| 100 | 200,870 | 0 | ~201,000 | 473 ms | 845 ms | 885 ms |

Reproduce with:

```bash
for c in 25 50 100; do
  go run ./cmd/wsload \
    -clients $c \
    -messages 20 \
    -settle-for 500ms \
    -send-interval 5ms \
    -drain-for 3s \
    -fail-on-loss
done
```

### Reading the numbers

- **Zero message loss** at every level — the hub's per-client write serialization holds under concurrent fan-out.
- Latency stays in **single-digit milliseconds at 25 clients** and grows with client count because broadcast is **O(clients²)**: 100 clients × 100 recipients per message is 10,000× the per-message work of a single send. This is the expected cost of in-process broadcast and is exactly why [SCALING.md](SCALING.md) routes multi-instance fan-out through a pub/sub layer rather than scaling the single-node hub.
- Throughput is reported as **delivered** frames, not attempted sends, so it reflects real server fan-out work.

## Notes

- Use a **fresh DB** for latency runs (`CHATSTER_DB_PATH=$(mktemp -u).db`). Otherwise history replay on connect re-delivers older messages whose timestamps inflate the latency distribution.
- For higher-volume runs, raise or disable `CHATSTER_MESSAGE_RPS` and `CHATSTER_MESSAGE_BURST`; the default limiter is intentionally demo-safe.
- Record machine, commit SHA, command, and date alongside any published numbers.
