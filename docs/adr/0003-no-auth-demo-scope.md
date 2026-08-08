# 3. No authentication in the reference scope

## Status

Superseded in part by ADR 0010

## Context

Adding mandatory OIDC/JWT/sessions increases frontend and backend surface area and distracts from the core WebSocket + persistence story for a portfolio-sized repo.

## Decision

Keep **anonymous mode** as the default public-demo behavior: display names are client-supplied text with no server-side identity proof. ADR 0010 adds an explicit opt-in signed-session mode for deployments that need stable identity and room authorization.

## Consequences

**Positive:** Faster comprehension of the real-time path; smaller codebase.

**Negative:** Anyone can impersonate any username; no per-user ACLs; abuse controls are limited to rate/size limits.

**Follow-up:** OIDC and server-side immediate session revocation remain optional extensions to ADR 0010.
