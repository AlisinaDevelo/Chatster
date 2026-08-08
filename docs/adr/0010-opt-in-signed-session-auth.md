# 10. Opt-in signed session authentication

## Status

Accepted

## Context

Chatster's anonymous demo lets a browser choose any display name. That keeps the public
demo small, but it cannot establish a stable user identity or enforce per-room access.
Browser WebSocket clients also cannot attach an arbitrary `Authorization` header during
the upgrade, so a bearer-only design would either exclude the browser client or place a
credential in the URL.

The first authentication mode needs to be self-contained, testable without an external
identity provider, and replaceable by OIDC later. It must not place credentials in the
frontend bundle, URLs, logs, or persisted chat rows.

## Decision

Keep `anonymous` as the default `CHATSTER_AUTH_MODE` for the public demo. Add an opt-in
`session` mode with these boundaries:

1. Operators provision high-entropy opaque access tokens (32-512 bytes), stable user IDs,
   display names, and up to 64 room grants through the secret
   `CHATSTER_AUTH_USERS_JSON` environment variable.
2. A browser exchanges an access token through `POST /api/session` using the
   `Authorization: Bearer` header. Tokens live only in process memory and are never
   returned, persisted, or logged.
3. The backend issues a short-lived, HMAC-SHA256 signed `HttpOnly`, `SameSite=Strict`
   cookie. The cookie binds a server-generated session ID, stable user ID, display name,
   allowed rooms, issue time, and expiry. Production cookies are `Secure` by default.
4. HTTP history and WebSocket upgrades validate the cookie and authorize the requested
   room server-side. Missing, invalid, and expired sessions receive `401`; a valid session
   without the room grant receives `403`.
5. Authenticated display names come from the signed session and cannot be changed through
   a WebSocket username message. Accepted chat rows retain the stable user ID separately
   from presentation data.
6. Credentialed history is `private, no-store`. Browser CORS reflects only an allowed
   Origin, and session mode refuses startup without an explicit Origin allowlist.

The signed-cookie service is the trust boundary inside Chatster. An OIDC callback or
identity-aware proxy can replace the opaque-token exchange later while issuing the same
server-side principal.

## Consequences

Positive: browser-compatible authentication, testable expiry and room authorization,
stable message ownership, no credential in WebSocket URLs, and no required identity
provider for local verification.

Negative: provisioned tokens and the signing secret require operator rotation. Sessions
are stateless, so logout deletes the browser cookie but cannot revoke another already
issued cookie immediately. Removing a provisioned token blocks new sessions; rotating the
signing secret revokes all sessions. Short expiry limits that window.

Follow-ups: add OIDC authorization-code flow with PKCE if the deployment needs public
accounts, and add a server-side session store only if individual immediate revocation is
required.
