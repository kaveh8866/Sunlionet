# Security Audit Checklist (Concrete, Repo-Backed)

See also:

- [Security Boundaries](boundaries.md) (trust boundaries + release-critical assets)
- [Release Checklist](../release/checklist.md) (ship gates)

This page avoids aspirational claims. Each item is either implemented today (with an enforcement point), or explicitly marked as a gap for later prompts.

## Implemented Today (Release-Critical)

- Bundle import verification is explicit and deterministic:
  - Strict wrapper/payload validation + canonicalization: `pkg/bundle/verify.go`
  - Importer enforces trusted signer allowlist and encrypted-only bundles: `pkg/importctl/import.go`
  - Import errors are typed (stable `Code*` values): `pkg/importctl/import.go`
- Profile normalization and conflict rejection is enforced:
  - `pkg/profile/normalize.go` and `pkg/importctl/import.go`
- Encrypted-at-rest Go stores exist and expose explicit secure-delete hooks:
  - Profile/template state: `pkg/profile/store.go`, `pkg/profile/template_store.go`
  - Other encrypted stores (identity/chat/community/devsync/ledger/aipolicy/comms) also implement `WipeOnSuspicion()` as a bounded recovery hook.
- Localhost dashboard surface has bind + redaction defenses:
  - `cmd/inside/runtime_api.go` sanitization and localhost binding
- CI runs secret scanning and Go tests on release verification:
  - `.github/workflows/reusable-verify-release.yml` (`gitleaks`, `go test ./...`)
 - Security review workflow exists and is restricted to trusted contexts:
  - `.github/workflows/security-review.yml` runs on manual dispatch or non-fork PRs only

## Review Checklist (Release Blockers)

- No plaintext secrets at rest where prohibited:
  - Verify all stores that persist sensitive state are encrypted (Go + Android).
- No secrets/tokens/seed material in logs:
  - Audit runtime API event payloads and Android logging paths for accidental leakage.
- Import validation is explicit and testable:
  - Ensure import entrypoints reject unknown signers, malformed payloads, and conflicts deterministically.
- Signer trust model is documented and matches code:
  - Keep [security/boundaries.md](boundaries.md) aligned with import requirements.
- Corrupted state has deterministic recovery behavior:
  - Verify behavior for missing/corrupt `.enc` files and partial writes is defined.
- Failed config/profile rotation has rollback behavior:
  - Ensure rotation doesn’t “accept” a broken config; enforce bounded retries.
- Hostile-network behavior has test hooks:
  - Use sim/integration harnesses to reproduce blocked DNS/UDP/reset behavior.
- Android permission/consent/security-sensitive flows are bounded:
  - Confirm VPN consent and import activation are user-mediated and fail closed.
- CI/release security review only runs in approved/trusted modes:
  - Verify that workflows using secrets (signing keys, Android keystore) are never run on untrusted forks/PRs.
- Release artifacts and docs are consistent:
  - The artifact list, verification steps, and download mirrors match the release workflows.

## Known Gaps (Must Be Resolved Before “Production” Claims)

- Replay protection persistence:
  - Current replay detection is process-lifetime only (`seenBundleIDs` is in-memory in `pkg/importctl/import.go`).
- Android gomobile artifact provenance:
  - The APK depends on checked-in gomobile artifacts under `android/app/libs/`; release CI does not rebuild them from Go sources.
- Log-leak regression tests:
  - Add automated “no secrets in logs/events” assertions across Go + Android paths.
