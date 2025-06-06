# Contributing

Thanks for helping improve Chatster.

## Getting started

1. Fork and clone the repository.
2. Install **Go 1.22+**, **Node 20+**, and (for `make lint`) [golangci-lint](https://golangci-lint.run/welcome/install/) on your PATH. CI runs the same linters if you skip local Go lint.
3. Run checks from the repo root:

```bash
make test
make lint
```

See [docs/WORKFLOWS.md](docs/WORKFLOWS.md) for CI details and [README.md](README.md) for how to run the app.

## Pull requests

- Open a PR against `main`.
- Keep commits **small and focused**; write **short** subject lines.
- Ensure CI is green (tests, lint, build).
- Update docs when you change behavior, configuration, or operational steps.

## Code style

- **Go**: `gofmt` / `go vet`; follow `golangci-lint` rules in `.golangci.yml`.
- **JavaScript / React**: CRA ESLint defaults; run `npm run lint` in `frontend/`.
- Prefer clear names and minimal scope; avoid unrelated refactors in the same PR.

## Conduct

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md).
