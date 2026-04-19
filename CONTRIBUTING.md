# Contributing

This project is security-sensitive. Please keep contributions minimal, testable, and easy to review.

## Development

- Go 1.25+
- Windows: PowerShell 7+ (use [run_tests.ps1](scripts/run_tests.ps1))
- Linux/macOS: use [run_tests.sh](scripts/run_tests.sh)

This repository pins its Go toolchain in [go.mod](file:///C:/Users/Kaveh/Desktop/Iran-Agent-Vpn/go.mod) via `toolchain`. If your local Go supports toolchains, `go` will auto-download the pinned version.

Before opening a PR:

1. Run unit tests.
2. Run `go vet ./...`.
3. Run `gofmt -w .`.

## Repository structure

- `cmd/inside` (build tag: `inside`): SunLionet Inside entrypoint
- `cmd/outside` (build tag: `outside`): SunLionet Outside entrypoint
- `pkg/*`: shared and Inside-only packages (see `/docs` for architecture)

## Security rules (strict)

- Do not add telemetry from Inside → Outside by default.
- The LLM must never receive secrets (keys, tokens, full configs).
- Bundle verification must remain fail-closed: invalid signatures must never be accepted.
- Avoid persistent logs on Inside (especially anything user-identifying).

## Translations

- Persian documentation is welcome.
- Keep translations in `README.fa.md` and `docs/fa/*`.
