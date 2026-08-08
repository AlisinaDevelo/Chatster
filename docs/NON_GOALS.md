# Explicit non-goals

Items **intentionally out of scope** for this reference repository. They may be valid product goals elsewhere.

- **Multi-tenant SaaS** billing, org management, and per-tenant isolation guarantees.
- **End-to-end encryption** of message contents (Signal-class threat model).
- **Mobile native clients** and push notification infrastructure.
- **Full-text search** and archival cold storage tiers.
- **Federation** (ActivityPub, XMPP bridges).
- **AI features** (moderation models, bots) beyond optional future hooks.
- **Self-service accounts and enterprise identity federation**; the opt-in session mode uses operator-provisioned tokens, while OIDC remains a future integration.
- **Replacing Vite** with a larger frontend framework without a concrete product need.

When a non-goal moves in-scope, add an **ADR** and update this list.
