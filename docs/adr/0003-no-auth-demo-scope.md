# 3. No authentication in the reference scope

## Status

Accepted

## Context

Adding OIDC/JWT/sessions increases frontend and backend surface area and distracts from the core WebSocket + persistence story for a portfolio-sized repo.

## Decision

Treat **display name** as client-supplied text with **no server-side identity proof**. Document risks in [THREAT_MODEL.md](../THREAT_MODEL.md).

## Consequences

**Positive:** Faster comprehension of the real-time path; smaller codebase.

**Negative:** Anyone can impersonate any username; no per-user ACLs; abuse controls are limited to rate/size limits.

**Follow-up:** Add session cookies or bearer tokens; bind `Client` to `user_id`; authorize room membership.
