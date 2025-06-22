# Threat model (reference)

Lightweight **STRIDE-style** view of Chatster as a **public demo**. It complements [SECURITY.md](../SECURITY.md) and implementation notes in [ARCHITECTURE.md](ARCHITECTURE.md).

## Assets

| Asset | Value / sensitivity |
|-------|---------------------|
| SQLite `messages` table | Integrity and availability of chat history; may contain PII (usernames, message text). |
| WebSocket sessions | Ability to send/receive as a connected “user” (no strong identity today). |
| API availability | DoS affects demo UX; cost if hosted on metered infra. |

## Trust boundaries

```text
[Browser / public Internet]  →  [Go HTTP + WS + SQLite on host/container]
```

- **Trusted:** operators with shell access to the host and DB file.
- **Untrusted:** anyone who can reach `:8080` / `wss://…` and the static frontend.

## Threats and mitigations

| ID | Threat | Mitigation in repo | Residual risk / follow-up |
|----|--------|-------------------|---------------------------|
| T1 | **Spoofing** — pretend to be another user | No cryptographic identity; display name is client-chosen. | Add **authn** (OIDC, session cookie, JWT) and server-side user id; bind WS to session. |
| T2 | **Tampering** — alter messages in transit | Use **TLS** (`https`/`wss`) at the edge; terminate TLS at reverse proxy or load balancer. | HSTS, cert rotation runbooks. |
| T3 | **Repudiation** — deny having sent a message | SQLite append-only style log; no signed audit trail. | Append-only store, signed events, or external SIEM if compliance requires it. |
| T4 | **Information disclosure** — leak data via logs, errors, CORS | Structured logs should avoid logging full message bodies at info level; restrict **CORS** and **WebSocket `Origin`** in prod (`CHATSTER_ALLOWED_ORIGINS`). | Log redaction policies; DLP review. |
| T5 | **Denial of service** — flood WS or HTTP | **WS upgrade rate limit** (per IP), **message size limits**, JSON-only small payloads; health checks for orchestration. | WAF, global rate limits, CAPTCHA at edge, autoscaling. |
| T6 | **Elevation of privilege** | Minimal surface: no admin API in demo. | If you add moderation/admin routes, enforce **authz** (RBAC) server-side. |

## Abuse scenarios (demo-appropriate controls)

| Scenario | Control |
|----------|---------|
| Huge JSON / long strings | Reject oversize **username** and **message** content server-side (runes/bytes limits). |
| Connection spam | Per-IP **rate limit on `/ws` upgrade** (configurable; disable locally if needed). |
| Open WebSocket origin | **Allowlist** `Origin` via `CHATSTER_ALLOWED_ORIGINS` when non-empty. |

## Out of scope for this reference

- Formal penetration test results.
- Compliance frameworks (SOC2, HIPAA, etc.).
- Key management for customer-managed keys.

Update this document when you add authentication, multi-tenant data, or a public deployment.
