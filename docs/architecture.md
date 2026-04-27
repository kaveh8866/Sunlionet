# System Architecture (Inside + Outside)

SunLionet is split into two coordinated agents compiled from one codebase.

## Production Targets (Release Boundary)

This repository’s public release boundary is defined by the release workflows and the default build tags:

- `sunlionet-inside` is built from `cmd/inside` with build tag `inside` (default build excludes `daemon`).
- `sunlionet-outside` is built from `cmd/outside` with build tag `outside`.

The product/repo map is captured in: [production-map.md](production-map.md).

Trust boundaries and enforcement points are captured in: [security/boundaries.md](security/boundaries.md).

## What Ships (Concrete Deliverables)

This repo currently ships multiple user/operator-facing surfaces:

- Go binaries:
  - Inside CLI/runtime (`cmd/inside`, `-tags inside`, default `!daemon`)
  - Outside CLI/tooling (`cmd/outside`, `-tags outside`, includes optional `relay` server mode)
- Android APK (`android/`):
  - Kotlin wrapper (VPN lifecycle + import UI + local diagnostics)
  - Embedded Go mobile API surface (gomobile artifacts referenced by `android/app/libs/`)
- Website + dashboard (`website/`):
  - Public docs/pages plus a local dashboard that proxies to the Inside localhost runtime API

## SunLionet Inside

Inside runs on the user device and owns all real-time decisions. It is designed to be lightweight, seizure-resistant, and able to operate during partial or total blackouts.

### Current Inside Runtime (Release)

Inside is implemented as a CLI/runtime that:

1. Imports a bundle from disk (or other channel via the Android wrapper), verifies it, and writes validated state to encrypted storage.
2. Selects and renders a sing-box outbound config from validated profiles/templates.
3. Optionally launches/validates sing-box and runs a probe-gated “connect” flow.
4. Optionally serves a localhost-only runtime API for the dashboard.

Concrete enforcement points live in:

- Bundle verification: `pkg/bundle` and `pkg/importctl`
- Encrypted state: `pkg/profile` (+ other `pkg/*/store.go`)
- sing-box generation/control: `pkg/sbctl`
- Local runtime API: `cmd/inside/runtime_api.go` (localhost-only binding + sanitization)

### Non-Release Variant (Experimental)

`cmd/inside/daemon.go` (build tag `inside && daemon`) is a separate autonomous-agent runtime path that integrates additional subsystems (LLM client, relay polling, ledger sync). It is not built by default and is not part of the public release boundary until explicitly hardened and gated.

### Android Agent Runtime (Release, via gomobile)

The Android app does not simply “wrap the Inside CLI”. It embeds a Go agent loop exposed via `pkg/mobilebridge` and invoked from Kotlin (`android/app/src/main/java/com/sunlionet/agent/Bridge.kt`).

Concrete responsibilities in the Android/Go combined runtime:

- Bundle import and encrypted storage: `pkg/mobilebridge` → `pkg/importctl`, `pkg/profile`, `pkg/profile.TemplateStore`
- Deterministic selection/rotation: `pkg/mobilebridge` → `pkg/policy`
- Optional local orchestrator (Pi) client: `pkg/mobilebridge` → `pkg/orchestrator` (local-only contract; treated as untrusted advisor)
- sing-box config generation + validation: `pkg/sbctl` (Go) and `SingBoxController.kt` (Android)

Note: `android/app/libs/` contains the gomobile artifacts that the APK links against; treat these as release-critical inputs (see [production-map.md](production-map.md)).

### Control Plane Components (Only Where They Exist Today)

Some subsystems exist as real code but intentionally return “unavailable” in real mode until a real implementation is introduced. This is a deliberate boundary: it prevents accidental reliance on a fake “real” mesh/Signal receiver.

- Detector: implemented for both real and sim modes (`pkg/detector/real`, `pkg/detector/sim`)
- Policy engine (deterministic): `pkg/policy`
- Secure local stores (encrypted-at-rest): `pkg/profile` and other `pkg/*/store.go`
- Mesh and Signal receiver:
  - Simulation implementations exist for testing (`pkg/mesh/sim`, `pkg/signalrx/sim`)
  - “Real” implementations currently return `unavailable` (`pkg/mesh/real`, `pkg/signalrx/real`)
- LLM advisor:
  - Present only in the non-release daemon build (`cmd/inside/daemon.go` uses `pkg/llm`)

## SunLionet Outside (Helper)

Outside runs in a safer jurisdiction. It continuously generates and tests new seed profiles and distributes bundles to Inside users via a one-way channel.

**Outside capabilities**

- Config factory: generate new profiles and templates (Reality, Hysteria2, TUIC v5, ShadowTLS v3, DNS tunnel fallback)
- Validation: test seeds against clean infrastructure and basic reachability checks
- Distribution: package profiles into signed/encrypted bundles and send over Signal
- Optional: run helper proxy instances and distribute access points (helper-controlled)

## Coordination (one-way by default)

Default data flow is Outside → Inside only:

1. Outside produces a bundle containing profiles/templates/metadata.
2. Outside signs (Ed25519) and encrypts the bundle payload.
3. Outside sends `snb://v2:<base64url(wrapper_json)>` via Signal.
4. Inside receives, verifies, decrypts, stores, and uses the bundle.

Inside may optionally send a minimal health report only if the user explicitly enables it. Default remains receive-only.

## LLM usage model (bounded advisor)

In the experimental daemon build (`inside && daemon`), Inside can use an LLM as an advisor only:

- The LLM never sees secrets or full configs.
- The LLM only chooses `profile_id` + allowed mutation set + cooldown.
- The policy engine applies the decision deterministically and fail-closed.

