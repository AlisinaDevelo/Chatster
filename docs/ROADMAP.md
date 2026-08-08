# Chatster roadmap

Chatster's current release baseline is a room-scoped chat service with SQLite history,
bounded WebSocket fan-out, graceful shutdown, opt-in tracing, Postgres repository
contracts, optional signed-session authorization, browser smoke coverage, and long-history
virtualization. The remaining work is
split between public-demo proof and optional platform extensions.

## Immediate execution order

1. **Deploy the public demo**: connect `main` to Render, provide the service origin through
   `CHATSTER_ALLOWED_ORIGINS`, and run the two-tab plus HTTP smoke checklist in
   [DEPLOYMENT.md](DEPLOYMENT.md). This is the only remaining release-proof step that
   requires an external account.
2. **Keep the single-node boundary honest**: retain one Render instance and the persistent
   `/data` disk while SQLite and the in-memory WebSocket hub are the active defaults.
3. **Reassess before scaling**: use the production demo's traffic and operational evidence
   to decide whether the implemented opt-in authentication, Postgres, and cross-instance
   fan-out modes should be enabled in a specific deployment.

## Optional platform tracks

- **Authentication** ([issue #28](https://github.com/AlisinaDevelo/Chatster/issues/28)):
  implemented as an opt-in signed-session boundary with stable user IDs and server-enforced
  room grants. Anonymous demo access remains the explicit default; OIDC and selective
  immediate revocation are future extensions.
- **Redis fan-out** ([issue #27](https://github.com/AlisinaDevelo/Chatster/issues/27)):
  implemented as an opt-in, namespaced live transport with CI-backed multi-instance coverage.
  Redis Pub/Sub is live fan-out, not durable replay; reconnect history remains the catch-up path.
- **Postgres mode** ([issue #25](https://github.com/AlisinaDevelo/Chatster/issues/25)):
  the repository mode and CI contract are ready; adopt it when shared durable storage is
  needed beyond the single-node SQLite demo.

The public-demo finish line is independent of those platform tracks. The demo should stay
small, observable, and honest about its single-instance limits until there is evidence to
justify the additional operational surface.
