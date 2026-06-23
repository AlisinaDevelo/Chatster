# Load Testing

Chatster includes a small Go WebSocket load generator so benchmark claims can be reproduced without extra tooling.

## Run

Start the backend:

```bash
cd backend
go run .
```

In another terminal:

```bash
cd backend
go run ./cmd/wsload -url ws://127.0.0.1:8080/ws -clients 25 -messages 20
```

The command prints a JSON summary with connected clients, attempted sends, received frames, send duration, and send rate.

## Notes

- `total_received` includes broadcasts, history replay, join/leave notifications, and private server notices.
- For higher-volume runs, raise or disable `CHATSTER_MESSAGE_RPS` and `CHATSTER_MESSAGE_BURST`; the default limiter is intentionally demo-safe.
- Record machine, commit SHA, command, and date alongside any published numbers.
