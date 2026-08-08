# Threat model (reference)

Lightweight **STRIDE-style** view of Chatster's public anonymous mode and opt-in signed-session
mode. It complements [SECURITY.md](../SECURITY.md), [ADR 0010](adr/0010-opt-in-signed-session-auth.md),
and the implementation notes in [ARCHITECTURE.md](ARCHITECTURE.md).

## Assets

| Asset | Value / sensitivity |
|-------|---------------------|
| Signing secret and provisioned access tokens | Credentials that can create or forge authenticated sessions; they must exist only in operator-controlled secret storage and process memory. |
| Signed browser session | Stable user ID, display name, room grants, session ID, and expiry. The cookie is integrity-protected but its payload is not encrypted. |
| `messages` table | Integrity and availability of room history; may contain PII in display names and message text. Authenticated rows also contain a stable user ID. |
| `moderation_audit_log` table | Abuse/debug evidence for rejected inputs; contains session IDs, optional stable user IDs, display names, bounded content previews, reasons, lengths, and timestamps. |
| WebSocket sessions | Ability to read and send in one room. Anonymous sessions have a server-generated opaque ID; authenticated sessions also carry a verified stable user ID and room grant. |
| API availability | DoS affects the chat experience and can create cost on metered infrastructure. |

## Modes and trust boundaries

`CHATSTER_AUTH_MODE=anonymous` is the explicit public-demo default. Display names are untrusted
presentation strings and all valid rooms are public. `CHATSTER_AUTH_MODE=session` requires an
operator-provisioned token exchange and a signed session for history and WebSocket access.

```text
[Operator secret manager]
       | signing secret + token/user/room grants
       v
[Browser] -- HTTPS/WSS --> [Go HTTP/WS + auth service] --> [SQLite/Postgres]
   |                              |
   | bearer token once            | signed HttpOnly cookie validation
   +------------------------------+
```

- **Trusted:** operators with access to deployment secrets and the selected database.
- **Untrusted:** browsers, public networks before TLS termination, message content, anonymous display names, room query parameters, and every cookie or token before server validation.
- **Enforced boundary:** the Go auth service signs and validates identity; HTTP history and the WebSocket upgrade independently authorize the normalized room.

## Threats and mitigations

| ID | Threat | Mitigation in repo | Residual risk / follow-up |
|----|--------|-------------------|---------------------------|
| T1 | **Spoofing**: claim another identity | Session mode binds an HMAC-signed stable `user_id`, display name, session ID, expiry, and room grants to each request. Authenticated WebSocket username overrides are rejected and audited. | Anonymous mode intentionally permits display-name impersonation. OIDC is needed for self-service or externally verified accounts. |
| T2 | **Tampering**: alter identity, grants, or messages in transit | Session payloads use HMAC-SHA256 and strict parsing; invalid signatures fail closed. TLS (`https`/`wss`) is required at the production edge. | Add HSTS and certificate-rotation runbooks at the hosting layer. The signed cookie is integrity-protected, not encrypted. |
| T3 | **Repudiation**: deny abusive input | Accepted authenticated messages persist stable user IDs. Rejected input persists session ID, optional user ID, display name, bounded preview, reason, length, and timestamp. | Events are not cryptographically signed; use append-only external logging or a SIEM if compliance requires stronger evidence. |
| T4 | **Information disclosure**: leak credentials or chat data | Access tokens are sent once in an `Authorization` header, held only in memory, hashed in the server lookup table, and never returned or logged. Cookies are `HttpOnly`, `Secure` by default, `SameSite=Strict`; credentialed history is `private, no-store`; responses use generic auth errors. | XSS can act as the logged-in browser even though it cannot read the cookie. Reverse proxies must redact auth headers. Add a reviewed CSP and regular dependency scanning for a higher-risk deployment. |
| T5 | **Denial of service**: flood HTTP, WebSockets, or message fan-out | Per-IP upgrade limits, per-client message limits, frame/content bounds, bounded outbound queues, repository timeouts, and health/metrics endpoints constrain the demo. | Add edge WAF/global limits, connection quotas, and capacity-based autoscaling for public production traffic. |
| T6 | **Elevation of privilege**: enter an ungranted room | Both `GET /api/messages` and `/ws` normalize the room and enforce the signed room grant server-side. Missing/expired sessions return `401`; denied rooms return `403`. | There is no admin surface or hierarchical RBAC. Add separate roles and authorization policy before introducing moderation/admin routes. |
| T7 | **CSRF and cross-site WebSocket use** | Session mode refuses startup without `CHATSTER_ALLOWED_ORIGINS`. HTTP CORS and WebSocket upgrades use the same allowlist; credentialed origins are reflected explicitly. `SameSite=Strict` blocks normal cross-site cookie sends, and login requires a bearer token. | Missing `Origin` is allowed for non-browser clients. Edge proxies must preserve `Origin`, terminate TLS correctly, and avoid broad origin rewriting. |
| T8 | **Stolen-cookie replay or stale authorization** | Sessions expire after a bounded `1m`-`24h` TTL; production cookies are `Secure`; logout expires the local cookie. Rotating the signing secret revokes all issued sessions. | Stateless sessions cannot selectively revoke one issued cookie, and room changes take effect after expiry. Add a server-side session/revocation store if immediate selective revocation is required. |

## Abuse scenarios

| Scenario | Control |
|----------|---------|
| Huge JSON / long strings | Reject oversize username and message content server-side using byte/rune limits. |
| Connection spam | Apply a per-IP token bucket before the WebSocket upgrade. |
| Message spam | Apply a per-client token bucket; rejected sends increment bounded-label metrics and write audit rows. |
| Authenticated display-name override | Reject the username frame, increment `identity_override`, and audit it against the stable user ID. |
| Repeated bad credentials | Return a generic `401` without revealing whether a user ID, token, or room exists. Edge-level login throttling remains recommended. |
| Open browser origin | Require an exact `CHATSTER_ALLOWED_ORIGINS` allowlist in session mode. |

## Session lifecycle

- Operators generate 32-512 byte high-entropy access tokens and a separate signing secret of at least 32 bytes; each configured user has at most 64 room grants.
- `POST /api/session` exchanges a bearer token for a short-lived signed cookie; the frontend clears its token input after the call.
- `GET /api/session` reports only mode and current principal state. It never returns a credential.
- `DELETE /api/session` expires the browser cookie. Removing a provisioned token prevents new sessions; rotating the signing secret invalidates all existing sessions.
- Authentication and authorization failures do not include token, cookie, user, or database details in logs or client responses.

## Out of scope

- Formal penetration-test or compliance certification claims.
- Self-service account recovery, MFA, OIDC, and enterprise federation.
- Selective immediate session revocation without adding server-side state.
- End-to-end encryption of message contents.

Update this document when identity providers, admin roles, multi-tenant data, or a public
authenticated deployment change these boundaries.
