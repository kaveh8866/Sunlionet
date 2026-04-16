# Trust Model (No Central Authority)

## Trust Goals

- Users can verify authenticity independently.
- Trust does not depend on a single maintainer, server, or domain.
- Releases and bundles remain verifiable even under takedowns.
- No telemetry, tracking, or hidden control mechanisms.

## What Users Should Be Able to Verify

- Source code is open and matches the published build inputs.
- Release artifacts are signed by multiple maintainers.
- Release manifests and checksums match downloaded binaries.
- Bundle verification (signatures, encryption) is enforced by the agent.

## Code Trust

### Open Source Reviewability

- Keep all core logic in public repositories with mirror strategy.
- Keep security-sensitive modules small, testable, and easy to audit.

### Reproducible Builds (Roadmap)

- Prefer deterministic build steps and documented build environments.
- Encourage independent builders to reproduce release artifacts and publish verification notes.

## Release Trust

### Signed Releases

- Every release must publish:
  - signed release artifacts
  - checksums
  - a release manifest describing what was built and from which commit
- Require at least 2 maintainer signatures to publish a release.

### Verification Steps

- Provide offline-verifiable instructions:
  - verify signature(s) over the manifest
  - verify artifact checksums match the manifest
  - verify the manifest references a public commit hash present in mirrors

## Bundle Trust

- Bundles are signed by publishers; the agent validates signatures locally.
- Bundle encryption uses `age` recipient keys; decryption happens on the user device.
- Revocations are applied locally and deterministically.

## Contributor Trust

### Principle: No Anonymous Privileged Access

- Contributors can be pseudonymous, but privileged operations are not anonymous:
  - release signing keys are owned by identifiable maintainers (public keys published)
  - SRG membership is public

### Review Requirements

- Sensitive modules require:
  - at least one SRG review
  - at least one maintainer approval
  - tests or reproducible verification steps

## Trust Decentralization Strategies

### Multiple Maintainers

- Maintain at least 3 active maintainers where possible.
- Rotate responsibilities to prevent operational knowledge concentration.

### Mirrors and Independent Distribution

- Maintain mirrors on multiple platforms (e.g., GitHub + GitLab + self-hosted git).
- Publish signing keys and verification instructions in every mirror.
- Encourage downstream distributions that preserve verification.

### Independent Builders

- Support a community process where independent builders:
  - build from the tagged commit
  - compare checksums
  - publish signed attestations

## Logging and Diagnostics Trust

- No telemetry collection and no background upload.
- Logs and reports must be user-controlled exports only.
- Diagnostics must avoid including identifiers, endpoints, or raw configs by default.

## Red Lines (Non-Negotiables)

- No telemetry, tracking IDs, or “anonymous identifiers”.
- No centralized kill switch or remote control mechanism.
- No hidden trust roots or private update channels.
- No release signing controlled by a single entity.

