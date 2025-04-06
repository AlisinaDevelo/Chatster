# Security

## Reporting a vulnerability

Please **do not** open a public issue for security-sensitive reports.

Instead, contact the maintainers privately (for example via GitHub Security Advisories for this repository, if enabled, or the contact method listed in the repository profile). Include:

- A short description of the issue and its impact
- Steps to reproduce (proof of concept if possible)
- Affected versions or commits, if known

We aim to acknowledge reports within a few business days.

## Scope notes

This project is a **demo / reference** stack. It uses permissive WebSocket origins in development and stores data in a local SQLite file. Before exposing it to the internet, review [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and harden configuration (TLS, origins, auth, backups, secrets).

## Supported versions

Security fixes are applied on the default branch (`main`) unless otherwise communicated.
