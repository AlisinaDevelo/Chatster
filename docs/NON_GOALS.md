# Explicit non-goals

Items **intentionally out of scope** for this reference repository. They may be valid product goals elsewhere.

- **Multi-tenant SaaS** billing, org management, and per-tenant isolation guarantees.
- **End-to-end encryption** of message contents (Signal-class threat model).
- **Mobile native clients** and push notification infrastructure.
- **Full-text search** and archival cold storage tiers.
- **Federation** (ActivityPub, XMPP bridges).
- **AI features** (moderation models, bots) beyond optional future hooks.
- **Replacing CRA** without an ADR and migration plan (see [FRONTEND.md](FRONTEND.md)).

When a non-goal moves in-scope, add an **ADR** and update this list.
