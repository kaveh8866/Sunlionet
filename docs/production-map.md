# Production Map (What Ships, What Is Production-Critical)

This repo is a monorepo with multiple deliverables built from one shared Go codebase plus an Android wrapper and a web dashboard/site.

## Official Release Artifacts (Public)

- `sunlionet-inside` (Go binary) built from `cmd/inside` with build tag `inside` (default build excludes `daemon`).
- `sunlionet-outside` (Go binary) built from `cmd/outside` with build tag `outside`.
- `app-release.apk` (Android) built from `android/`.
- Checksums + signed checksum bundle (CI release flow): `checksums.txt`, `checksums.sig`, `checksums.pub`, `checksums.pub.sha256`, `release.json`.

## Repo Layout: Authoritative vs Support

### Production-Critical (Release-Boundary Code)

- `cmd/inside/` (Inside CLI/runtime)
  - Includes: bundle import, profile selection/rotation, sing-box config rendering/validation/launch, optional localhost runtime API.
  - Excludes from release by default: `cmd/inside/daemon.go` (`//go:build inside && daemon`).
- `cmd/outside/` (Outside CLI/tooling)
  - Includes: keygen, bundle generation, bundle verification, optional relay server mode.
- `pkg/` (shared core libraries)
  - Import + verification: `pkg/bundle`, `pkg/importctl`
  - Encrypted state: `pkg/profile` (+ other `pkg/*/store.go` encrypted stores)
  - Runtime decision path: `pkg/orchestrator`, `pkg/policy`, `pkg/detector`, `pkg/sbctl`
  - Mobile bridge: `pkg/mobile`, `pkg/mobilebridge` (gomobile-facing APIs)
- `android/` (Android wrapper app)
  - `android/app/src/main/…` is authoritative runtime wrapper code (`VpnService`, secure storage, import UI).
- `website/` (Dashboard + public website)
  - `website/src/…` is authoritative UI/runtime proxy code.
  - `website/public/downloads/…` is the static download directory used by the site.
- `.github/workflows/` (release and verification enforcement)
  - Release build + verify + publish workflows define what “ships” and what gates a release.

### Support / Dev / Test (Not Shipped as Runtime Components)

- `tests/` (integration/simulation/security/perf harnesses; invoked by CI and local dev)
- `scripts/` (local build/test/release helpers; not used by shipped binaries at runtime)
- `packaging/` (packaging templates for CI; not executed by shipped binaries at runtime)
- `docs/` (documentation content published via GitHub Pages)
- `content/` (website content inputs for Next.js routing and localization)
- `Brandkit/`, `assets/` (branding assets)

## User-Facing vs Operator-Facing Surfaces

- User-facing:
  - Android app UI (`android/app/src/main/…`)
  - `sunlionet-inside` CLI (Linux/desktop MVP)
  - Documentation (`docs/`)
- Operator/developer-facing:
  - `sunlionet-outside` CLI (bundle generation/verification)
  - Optional relay server mode (`sunlionet-outside relay …`)
  - Local dashboard UI (`website/…`) reading from localhost runtime API

## Explicit Non-Release Targets (Current)

- `cmd/inside/daemon.go` (`inside && daemon`): a separate autonomous-agent runtime path (LLM/relay/ledger sync integration). It is not built by default and is not part of the public release boundary until explicitly hardened and gated.
