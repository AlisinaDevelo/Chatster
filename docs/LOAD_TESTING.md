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
  -settle-for 1s \
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
- `settle_for` — startup grace period after username setup; this lets the hub register every WebSocket before the timed send window.
- `send_interval` — per-client pacing between messages; use a non-zero value for a sustained run instead of a single microburst.

## CI smoke

The GitHub Actions backend job starts a local Chatster server with WS/message rate limits disabled and runs:

```bash
go run ./cmd/wsload \
  -url ws://127.0.0.1:18080/ws \
  -clients 4 \
  -messages 3 \
  -settle-for 250ms \
  -send-interval 10ms \
  -drain-for 1s \
  -fail-on-loss
```

The JSON result is uploaded as `wsload-smoke.json` in the `backend-runtime-proof` artifact. This is intentionally small and non-flaky; benchmark-scale runs should remain manual and machine-labeled.

## Results

Measured on an **Apple M1 (8 cores), macOS 26.5.1, Go 1.26.3**, 2026-06-24, with both rate limiters disabled and a **fresh SQLite database per run**. Each client sent 20 messages after a 1 s settle period, paced at 5 ms; every message is broadcast to all connected clients, so deliveries scale as O(clients²). The 25-client row is a single representative run; the 50-client row is the **median of 5 runs**.

| Clients | Delivered | Lost | Throughput (deliveries/s) | p50 | p95 | p99 |
|--------:|----------:|-----:|--------------------------:|----:|----:|----:|
| 25 | 12,500 / 12,500 | 0 | ~121,000 | 2.2 ms | 4.3 ms | 5.7 ms |
| 50 | 50,000 / 50,000 | 0 | ~314,000 | 22 ms | 48 ms | 57 ms |

Reproduce with:

```bash
for c in 25 50; do
  go run ./cmd/wsload \
    -clients $c \
    -messages 20 \
    -settle-for 1s \
    -send-interval 5ms \
    -drain-for 6s \
    -fail-on-loss
done
```

### Reading the numbers

- **Zero message loss** at 25 and 50 clients across every run — the hub's per-client write serialization and bounded outbound queues hold under concurrent fan-out.
- Latency stays in **single-digit milliseconds at 25 clients** and grows with client count because broadcast is **O(clients²)**: each message is delivered to every connected client, so per-message work scales with the square of the client count. This is the expected cost of in-process broadcast and is exactly why [SCALING.md](SCALING.md) routes multi-instance fan-out through a pub/sub layer rather than scaling the single-node hub.
- Throughput is reported as **delivered** frames, not attempted sends, so it reflects real server fan-out work. The harness deduplicates each `(sender, sequence)` per recipient so history replay cannot inflate delivery counts.

### Why no 100-client row

A single node is past its useful broadcast ceiling well before 100 clients, and the numbers there are **not reproducible** on commodity hardware: across five 100-client runs we saw delivery shortfalls ranging from ~1% to ~50% and noticeable client-side send aborts under load. The shortfall is *not* bounded-queue shedding (`chatster_websocket_outbound_drops_total` stayed at 0) — it is the single-node O(N²) fan-out saturating connection handling. Rather than publish a precise-looking but unstable figure, the takeaway is qualitative: **in-process broadcast does not scale past a few dozen concurrent clients**, which is the motivation for the pub/sub design in [SCALING.md](SCALING.md). Run `wsload` at higher client counts yourself to see the variance.

## Notes

- Use a **fresh DB** for latency runs (`CHATSTER_DB_PATH=$(mktemp -u).db`). Otherwise history replay on connect re-delivers older messages whose timestamps inflate the latency distribution.
- Keep `-settle-for` non-zero for benchmark runs; otherwise a fast client can send messages before every WebSocket has been registered by the hub.
- For higher-volume runs, raise or disable `CHATSTER_MESSAGE_RPS` and `CHATSTER_MESSAGE_BURST`; the default limiter is intentionally demo-safe.
- Record machine, commit SHA, command, and date alongside any published numbers.
