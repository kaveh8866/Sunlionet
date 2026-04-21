# Release Checklist (Release Blockers + Publication)

This checklist is for preparing and publishing a public SunLionet release (example: `v0.1.0`).

## Release Blockers (Cannot Ship Unless True)

### Secrets and Sensitive Data
- No plaintext secrets at rest where prohibited:
  - Go state stores remain encrypted-at-rest (AES-GCM) and never write plaintext profiles/templates/identity to disk.
  - Android uses keystore-backed encrypted storage for app secrets and state.
- No secrets/tokens/seed material in logs:
  - Go runtime API events remain sanitized and bounded.
  - Android logs are bounded/sanitized and release builds do not ship verbose logging enabled.
- Repo passes secret scanning:
  - `gitleaks` passes in CI for the release tag.

### Bundle Import and Trust
- Import validation is explicit and testable:
  - Signature verification is fail-closed against an explicit trusted signer allowlist.
  - Payload canonicalization and strict validation is enforced.
  - Replay and conflict behavior is deterministic (duplicate IDs/endpoints rejected).
- Signer trust model is documented:
  - Trust anchors, signing meaning, and transport assumptions are described in docs.

### State Corruption and Recovery
- Corrupted state has deterministic recovery behavior:
  - Encrypted stores and runtime state handle missing/corrupt files without undefined behavior.
- Failed config/profile rotation has rollback behavior:
  - A failed profile attempt does not silently “accept” a broken config.
  - Max-attempt limits prevent thrashing and loops.

### Hostile-Network Behavior
- Hostile-network behavior has test hooks:
  - Simulation and integration tests exist to exercise degraded/blocked network behavior.
  - Real-mode smoke tests exist for basic connect/import flows (where supported).

### Android Security Boundaries
- Android permission/consent/security-sensitive flows are bounded:
  - VPN consent is explicit user action.
  - Import UI does not accept unverified/untrusted bundles as “active”.
  - Stored secrets remain in encrypted storage (no plaintext fallbacks).

### CI / Release Boundary
- CI security review tooling only runs in approved/trusted modes:
  - LLM-based security review workflow is restricted to internal branches/PRs (fork PRs are skipped).
- Release artifacts and docs are consistent:
  - Artifact names, install instructions, and verification steps match what CI produces.

## Artifacts

- SunLionet Inside artifacts built for linux/amd64, linux/arm64, darwin/arm64, windows/amd64
- SunLionet Outside artifacts built for linux/amd64, linux/arm64, darwin/arm64, windows/amd64
- Android APK built and signed (`app-release.apk`)
- Checksums generated for every artifact (`*.sha256` or `checksums.txt`)
- Checksum signatures generated and verified (`checksums.sig` + `checksums.pub`, if used)

## Verification

- Linux install path tested:
  - tarball extract + `install-linux.sh` works
  - systemd service installs and starts (if used)
- Bundle import tested (Inside):
  - valid bundle imports successfully
  - invalid signature fails closed with a clear error
- Connection test executed (Inside):
  - probe success path works
  - probe failure path produces a readable reason
- Dashboard tested:
  - runtime API starts on localhost only
  - `/dashboard/runtime` shows status + active profile + failures
- Android tested:
  - install + VPN permission flow works
  - import bundle flow works
  - connect/disconnect toggles state
  - last error is visible when failures occur

## Documentation

- README.md points to docs/getting-started.md as the single entry point
- Installation docs accurate for Linux and Android
- Bundle usage docs accurate (generate/verify/import)
- Troubleshooting covers common failure modes (missing keys, invalid signer key, sing-box missing)
- Security model describes trust boundaries without exaggerated claims

## Release process

- Version string set in build (`-X main.version=vX.Y.Z`)
- Tag created: `git tag vX.Y.Z`
- CI release workflow runs successfully for the tag
- GitHub release created with:
  - binaries + APK
  - checksums and signature metadata
  - short install instructions and verification steps
