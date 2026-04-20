# Developer Setup

## Required Go Version

- Required: Go `1.24.0` (see `.go-version`)

## Install Go

1. Install Go `1.24.0` from [go.dev](https://go.dev/dl/).
2. Verify your toolchain:

```bash
go version
```

Expected output should contain `go1.24.0`.

## Bootstrap Local Environment

From repository root:

```bash
bash scripts/dev-setup.sh
```

This downloads modules and runs the test suite.

## Build and Test Manually

```bash
go mod tidy
go build ./...
go test ./...
go vet ./...
```

Or use:

```bash
make build
make test
make lint
```

## Version Injection for CLI

Build `shadownet-inside` with an explicit version string:

```bash
go build -ldflags "-X main.version=0.1.0" -tags inside -o bin/shadownet-inside ./cmd/inside
```

## Troubleshooting

- `toolchain not available`:
  Install Go `1.24.0` locally and rerun commands; avoid relying on auto-download.
- `go.mod/go.sum changed in CI`:
  Run `go mod tidy` locally and commit the resulting `go.mod` and `go.sum`.
- `tests fail only in CI`:
  Check `go version` locally and ensure it matches CI (`1.24.x`/`1.25.x`).
- Offline development:
  Warm module cache once (`go mod download`) before going offline.
