---
id: 0010
title: Bind authenticated identity to sessions and room authorization
status: done
agent: security-auditor
model: opus
depends_on: []
parent: 0001
---

## Goal
Move beyond client-chosen display names by defining an authentication and authorization path that can bind a WebSocket session to a stable user identity and permitted rooms.

## Acceptance criteria
- [x] Add a threat-model/ADR decision for the first auth mode (OIDC-backed session cookie, bearer token, or another reviewed option) and its trust boundaries.
- [x] Bind the authenticated user ID to HTTP history and WebSocket sessions; display names remain presentation data rather than identity.
- [x] Enforce room authorization server-side and test allowed, denied, expired, and anonymous-demo behavior.
- [x] Keep secrets out of the frontend bundle and logs; document CSRF, token expiry, logout/revocation, and Origin/TLS requirements.
- [x] Update the public-demo scope explicitly so anonymous mode is either preserved as a deliberate feature flag or removed with a migration note.

## Context
`docs/THREAT_MODEL.md` and `docs/adr/0003-no-auth-demo-scope.md` currently name authentication as out of scope. This is a larger security track and should not be mixed into Redis or Postgres implementation work.

## Notes
Use the threat model as the baseline and require a security review before implementation. No provider or credential should be selected from an issue comment without an explicit design decision.

## Verification

- `CHATSTER_REDIS_TEST_URL=redis://127.0.0.1:6379/15 go test -race -coverprofile=/tmp/chatster-coverage.out ./...` passed with 61.1% aggregate statement coverage.
- `go vet ./...`, frontend ESLint, 30 Vitest tests, the Vite production build, and three Chromium Playwright workflows passed locally.
- GitHub Actions run [31236267397](https://github.com/AlisinaDevelo/Chatster/actions/runs/31236267397) passed the pinned golangci-lint gate, race-enabled Redis suite, Postgres repository contract, frontend browser workflows, and production-image smoke.
