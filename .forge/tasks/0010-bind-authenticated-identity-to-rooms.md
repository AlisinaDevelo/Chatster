---
id: 0010
title: Bind authenticated identity to sessions and room authorization
status: backlog
agent: security-auditor
model: opus
depends_on: []
parent: 0001
---

## Goal
Move beyond client-chosen display names by defining an authentication and authorization path that can bind a WebSocket session to a stable user identity and permitted rooms.

## Acceptance criteria
- [ ] Add a threat-model/ADR decision for the first auth mode (OIDC-backed session cookie, bearer token, or another reviewed option) and its trust boundaries.
- [ ] Bind the authenticated user ID to HTTP history and WebSocket sessions; display names remain presentation data rather than identity.
- [ ] Enforce room authorization server-side and test allowed, denied, expired, and anonymous-demo behavior.
- [ ] Keep secrets out of the frontend bundle and logs; document CSRF, token expiry, logout/revocation, and Origin/TLS requirements.
- [ ] Update the public-demo scope explicitly so anonymous mode is either preserved as a deliberate feature flag or removed with a migration note.

## Context
`docs/THREAT_MODEL.md` and `docs/adr/0003-no-auth-demo-scope.md` currently name authentication as out of scope. This is a larger security track and should not be mixed into Redis or Postgres implementation work.

## Notes
Use the threat model as the baseline and require a security review before implementation. No provider or credential should be selected from an issue comment without an explicit design decision.
