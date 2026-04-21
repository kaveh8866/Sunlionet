# Contributing

## Quick Start (Devs)

### Prereqs

- Go toolchain
- Git

### Build

```bash
go build -tags inside -o bin/sunlionet-inside ./cmd/inside/
go build -tags outside -o bin/sunlionet-outside ./cmd/outside/
```

### Test

```bash
go test ./...
```

### Lint

```bash
go vet ./...
go vet -tags outside ./...
```

### Local Scenarios

- Use `tests/simulation` for protocol and DPI simulations.
- Use `tests/integration` for end-to-end bundle and runtime checks.

## Contribution Workflow

- Small PRs with focused scope.
- Include tests for behavioral changes.
- Update docs for user-visible changes.

## Security and Privacy Rules

- Never add telemetry, analytics, or tracking identifiers.
- Never log secrets, keys, endpoints, or raw configs.
- Be careful with “helpful debugging” that increases user risk.
- Changes touching crypto, update verification, or distribution require extra review.

## Coding Standards

- Run `gofmt` before pushing.
- Prefer explicit error handling and fail-safe defaults.
- Keep state minimal and encrypted by default.

## Reporting Security Issues

- Do not open a public issue for exploit details.
- Prefer private contact channels listed in the repository.
