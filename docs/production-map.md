# Production Map (What Ships, What Is Production-Critical)

This repo is a monorepo with multiple deliverables built from one shared Go codebase plus an Android wrapper and a web dashboard/site.

## Official Release Artifacts (Public)

The public release boundary is defined by `.github/workflows/release.yml` and its reusable build/verify/publish workflows.

Primary artifacts produced by release CI (examples, not an exhaustive list):

- SunLionet Inside (`cmd/inside`, tag `inside`, default excludes `daemon`)
  - `sunlionet-inside-<version>-linux-amd64.tar.gz` (+ `.sha256`)
  - `sunlionet-inside-<version>-linux-arm64.tar.gz` (+ `.sha256`)
  - `sunlionet-inside-<version>-darwin-arm64.tar.gz` (+ `.sha256`)
  - `sunlionet-inside-<version>-windows-amd64.zip` (+ `.sha256`)
  - `sunlionet-inside-<version>-android-arm64` (+ `.sha256`) (raw Termux/Android CLI binary; not the APK)
- SunLionet Outside (`cmd/outside`, tag `outside`)
  - `sunlionet-outside-<version>-linux-amd64.tar.gz` (+ `.sha256`)
  - `sunlionet-outside-<version>-linux-arm64.tar.gz` (+ `.sha256`)
  - `sunlionet-outside-<version>-darwin-arm64.tar.gz` (+ `.sha256`)
  - `sunlionet-outside-<version>-windows-amd64.zip` (+ `.sha256`)
- Combined quick-install tarball (Linux amd64):
  - `sunlionet-linux-amd64.tar.gz` (+ `.sha256`)
- Source snapshot:
  - `sunlionet-agent-<version>-source.tar.gz` (+ `.sha256`)
- Android:
  - `app-release.apk` and `sunlionet-android-<version>-app-release.apk` (+ `.sha256`)
- Optional packages (when enabled in CI):
  - `sunlionet_<version-without-v>_amd64.deb` (+ `.sha256`)
  - `sunlionet-<version-without-v>.x86_64.rpm` (+ `.sha256`, optional)
- Release verification metadata (produced by verify workflow):
  - `checksums.txt`, `checksums.sig`, `checksums.pub`, `checksums.pub.sha256`, `release.json`

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
  - Runtime decision path: `pkg/policy`, `pkg/detector`, `pkg/sbctl`, `pkg/runtimecfg`
  - Android/mobile bridge: `pkg/mobilebridge` + `pkg/mobile` (gomobile-facing API surface used by `android/`)
  - Outside relay server: `pkg/relay` (HTTP server + storage + traffic-shaping primitives)
- `android/` (Android wrapper app)
  - `android/app/src/main/…` is authoritative runtime wrapper code (`VpnService`, secure storage, import UI).
  - `android/app/libs/` contains checked-in gomobile artifacts (`sunlionet.aar`, `sunlionet-sources.jar`) that the APK depends on.
- `website/` (Dashboard + public website)
  - `website/src/…` is authoritative UI/runtime proxy code.
  - `website/public/downloads/…` is the static download directory used by the site (a mirror of selected release assets; not the canonical release of record).
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

## Supply Chain Note (Android gomobile artifacts)

The Android APK is built from `android/` in release CI, but it depends on prebuilt gomobile artifacts checked into this repo under `android/app/libs/`. This is a deliberate, explicit trust decision:

- Production-critical implication: shipping the APK without regenerating/verifying these artifacts can silently ship Go code that does not match the current commit.
- Current enforcement point: the release workflows do not rebuild `sunlionet.aar` from Go sources.
- Release constraint for production: treat `android/app/libs/*` as a release-critical input that must be audited, checksummed, and kept in sync with the Go source that defines the intended behavior.
