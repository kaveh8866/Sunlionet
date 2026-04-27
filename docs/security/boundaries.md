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

Important concrete constraint (today): the Go importer currently requires decryption on import (`RequireDecrypt: true` in `pkg/importctl/import.go`). Plaintext bundles may exist at the schema level, but they are not accepted by the current Inside/Android import path.

## Release-Critical Assets and Invariants (Concrete)

This section is the “what must never be mishandled” inventory for Prompt 2 enforcement work. It describes where assets enter, where they live, and what must never happen.

| Asset | Where it enters | Where it is stored | Validation / enforcement | Must never happen | Tests to enforce |
|---|---|---|---|---|---|
| Trust bundles / seed bundles (`snb://v2:` wrapper bytes) | `pkg/importctl.Importer.ParseURI/ParseBytes`, `cmd/outside verify`, Android `Bridge.importBundle` | Not stored as raw bundle; only validated payload is written | Signature + schema + canonicalization: `pkg/bundle/verify.go`; import normalization/conflict checks: `pkg/importctl/import.go` | Accepting unsigned/unknown-signer bundles; accepting non-canonical payloads; accepting conflicting duplicates | `pkg/bundle/*_test.go`, `pkg/importctl/import_test.go` |
| Trusted signer allowlist (public keys) | CLI flags / Android SecureStore setting | In-memory allowlist; Android stores user-provided trusted signer keys | Key ID derivation + allowlist check: `pkg/bundle.Ed25519KeyID`, `pkg/importctl/import.go` | Treat transport as the trust anchor; silently accepting unknown signers | Add tests for empty allowlist failure and unknown signer failure paths in import entrypoints |
| Decryption identity (age X25519) | CLI flags / Android SecureStore | In-memory after parsing; Android stores in keystore-backed encrypted storage | Decrypt required on import: `pkg/importctl/import.go` (`RequireDecrypt: true`) | Accepting encrypted payload without an identity; falling back to plaintext silently | Tests for “cipher none” refusal (if supported by schema) + wrong identity behavior |
| Master key (local encryption key) | CLI flags / Android SecureStore | In-memory; never persisted | Key parsing/length checks: `pkg/profile.ParseMasterKey` and store constructors | Writing master key to disk/logs; using weak/short keys | `pkg/profile/store_test.go` + add log-scan tests on error paths (Prompt 2) |
| Encrypted profile/template state (Go) | Output of verified import | `profiles.enc`, `templates.enc` under the configured state dir | AES-GCM encryption + restrictive perms: `pkg/profile/store.go`, `pkg/profile/template_store.go` | Plaintext profile/template data at rest; world-readable perms | `pkg/profile/store_privacy_test.go`, `pkg/profile/store_wipe_test.go` |
| Android stored secrets/state | App bootstrap + user actions | Keystore-backed encrypted storage | `android/app/src/main/java/com/sunlionet/agent/SecureStore.kt` | Plaintext fallback for secrets; leaking secrets to logcat | Extend `androidTest` to assert release logging/off-by-default and no plaintext prefs (Prompt 2) |
| Local runtime events/state (localhost API) | Inside runtime event pipeline | In-memory buffer + API response JSON | Localhost bind + sanitization: `cmd/inside/runtime_api.go` | Exposing API on non-local interfaces; leaking secrets/full configs in events | `cmd/inside/runtime_api_sse_test.go` + add negative tests for file/link/secrets redaction (Prompt 2) |
| Logs/diagnostics (Go + Android) | Runtime | Filesystem logs (sing-box stdout/stderr), Android Logs | Sanitization helpers and bounded output; avoid raw error dumping | Emitting bundle contents, keys, identities, tokens, or full endpoint lists | Expand log-redaction tests (`pkg/assistant/redact.go`, runtime API sanitizers) (Prompt 2) |
| Release signing materials (cosign keys) | GitHub Actions secrets | GitHub Actions only | `reusable-verify-release.yml` signs `checksums.txt` to `checksums.sig` | Using signing keys on untrusted PRs/forks; leaking keys into logs/artifacts | CI policy: never run signing jobs on fork PRs; add explicit guards (Prompt 2) |
| Release verification metadata (`checksums.*`, `release.json`) | Release verify workflow | GitHub release assets; mirrored into website downloads in some workflows | Cosign verify step in `reusable-verify-release.yml` | Publishing assets without offline-verifiable checksums/signature | Add a CI job that downloads the just-built release artifact set and verifies it offline (Prompt 2) |
| Website local download mirror (`website/public/downloads/...`) | Release mirroring / local scripts | Git repo (static files) | Content parity checker: `website/scripts/content-check.mjs` | Serving mismatched artifacts vs hashes; serving stale/unverified mirrors as “official” | `npm run content:check` in website CI |

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
  - The current importer requires encrypted payloads on import; any future plaintext-support must be an explicit, reviewed decision with a separate threat analysis.

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
