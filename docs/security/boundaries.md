# Security Architecture: Trust Boundaries and Enforcement Points

This document maps the repo’s real trust boundaries to concrete enforcement points in code and CI. It is intentionally specific to what exists today.

## System Zones (High-Level)

- Outside operator zone (safer): generates bundles, holds signing keys, optionally runs relay services.
- Transport zone (untrusted): any messenger/file/QR/offline transfer used to deliver bundles.
- Inside device zone (hostile): imports bundles, stores state, renders configs, and drives runtime decisions.
- Android wrapper zone (hostile + user UI): handles VPN lifecycle, QR/file import UI, and secure local storage.
- Localhost dashboard zone (same device): reads a localhost-only runtime API for operator visibility.
- CI/release zone (trusted automation): produces and signs release artifacts; runs security scanning.

## Release-Critical Security Assumptions

- Transport is not trusted for integrity or confidentiality.
  - Integrity must come from Ed25519 signatures; confidentiality (if required) comes from age encryption.
- Trusted signer public keys are provisioned out-of-band and are treated as the primary trust anchor.
- Inside and Android devices are assumed seizable; local state must be encrypted-at-rest and logs must not leak secrets.
- The localhost runtime API is a local-only diagnostic surface and must refuse non-local binds.
- LLM-based review tooling is treated as untrusted and is never run on untrusted fork PRs.

## Trust Boundaries (Concrete)

### 1) Bundle Import Boundary (Untrusted → Verified)

- Entry points:
  - Inside import flow: `pkg/importctl` and `pkg/bundle`.
  - Outside verification tooling: `cmd/outside verify` and `pkg/bundle`.
- Enforcement points:
  - Strict wrapper/header/payload verification and canonicalization: `pkg/bundle/verify.go`.
  - Import-time profile conflict checks and normalization: `pkg/importctl/import.go`.
- Security invariants:
  - Reject invalid signatures and unknown signers (fail closed).
  - Reject malformed/expired bundles and non-canonical payloads.
  - Reject conflicting duplicates (IDs/endpoints) deterministically.

### 2) Signer / Key Trust Boundary (Trust Anchors)

- Trust anchors:
  - Ed25519 trusted signer public keys (Inside allowlist).
  - age X25519 identity (Inside decrypt capability, if encrypted payloads are required).
- Enforcement points:
  - Bundle signature verification against a specific trusted signer key: `pkg/bundle/verify.go`.
  - Importer allowlist of trusted signer key IDs: `pkg/importctl/import.go`.
  - Outside trust model documentation: `docs/outside/trust-model.md`.
- Security invariants:
  - A bundle “from the transport” is not trusted until it verifies under an allowlisted signer.
  - Encryption is optional by schema (`cipher="none"` exists) but must be an explicit decision; CI can enforce `--require-decrypt` for verification when confidentiality is required.

### 3) Encrypted Storage Boundary (Secrets at Rest)

- Assets stored encrypted-at-rest (Go):
  - Profiles/templates/state: `pkg/profile/store.go`, `pkg/profile/template_store.go`.
  - Identity, chat, ledger, comms, policy stores: `pkg/identity/store.go`, `pkg/chat/store.go`, `pkg/ledger/store.go`, `pkg/comms/store.go`, `pkg/aipolicy/store.go`, `pkg/devsync/store.go`, `pkg/community/store.go`.
- Enforcement points:
  - Master key parsing and validation: `pkg/profile/store.go` (`ParseMasterKey`).
  - Disk writes are ciphertext-only (AES-GCM), file permissions are restricted (`0600` / `0700`).
- Android encrypted storage boundary:
  - Keystore-backed encrypted preferences: `android/app/src/main/java/com/sunlionet/agent/SecureStore.kt`.
- Security invariants:
  - No plaintext seed material is written to disk by the Go stores.
  - Android must not introduce plaintext fallbacks for secrets/state.

### 4) Network / Session Boundary

- Inside runtime network boundary:
  - sing-box process control and config generation live in `pkg/sbctl` and `cmd/inside`.
  - Runtime modes are explicit (real vs simulation): `pkg/runtimecfg` + CLI flags.
- Outside network boundary:
  - Optional relay server: `cmd/outside` (`relay` subcommand) and `pkg/relay`.
- Security invariants:
  - No outbound telemetry by default (Inside → Outside is not required for core operation).
  - Network-facing server modes (relay) must have explicit rate limits, storage bounds, and safe defaults.

### 5) Android App Boundary

- Responsibilities:
  - Owns VPN lifecycle and user consent, config import UI, and secure local state.
- Enforcement points:
  - VPN entrypoint: `android/app/src/main/java/com/sunlionet/agent/SunlionetVpnService.kt`.
  - Sensitive storage: `android/app/src/main/java/com/sunlionet/agent/SecureStore.kt`.
  - Runtime bridge into Go: `android/app/src/main/java/com/sunlionet/agent/Bridge.kt` (gomobile-facing).
- Security invariants:
  - VPN permission is user-mediated and must not be bypassed.
  - Import UI must not “activate” unverified bundles.

### 6) Local Dashboard / Localhost Boundary

- Contract:
  - Dashboard reads state from a localhost-only runtime API exposed by `sunlionet-inside`.
- Enforcement points:
  - Localhost bind enforcement: `cmd/inside/runtime_api.go` refuses non-local hosts.
  - Runtime event sanitization: `cmd/inside/runtime_api.go` (`sanitizeRuntimeText`, `sanitizeRuntimeMeta`).
  - Dashboard integration notes: `docs/dashboard/runtime-integration.md`.
- Security invariants:
  - Runtime API must never bind to non-local interfaces by default or by accident.
  - Returned data remains operational metadata only (no secrets, no full configs).

### 7) CI / Release Boundary

- What defines “what ships”:
  - Release workflows: `.github/workflows/release.yml` + reusable build/verify/publish workflows.
- Enforcement points:
  - Secret scanning: gitleaks in CI/release verification.
  - Go vulnerability scanning: govulncheck in CI.
  - Checksums + signature generation for releases: `.github/workflows/reusable-verify-release.yml`.
- Security invariants:
  - Public releases must be verifiable offline from published checksums + signatures.
  - CI must not require secrets to run routine tests on untrusted contributions.

### 8) External Contributor / Untrusted PR Boundary

- Threat:
  - Prompt injection and data exfiltration risks from LLM-based review automation.
- Enforcement points:
  - LLM-based review automation (if enabled) must be gated so secrets are never exposed to untrusted forks/PRs.
- Security invariants:
  - Never run LLM review tooling with secrets on untrusted forks/PRs.
  - Treat all LLM output as advisory; merges require human review.
