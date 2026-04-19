# Governance Model (Lightweight, Resilient)

## Goals

- Keep the project survivable beyond any single developer.
- Keep decision-making transparent and reviewable.
- Avoid single points of trust or control.
- Preserve privacy-first and decentralization constraints.

## Roles

### Community Members

- Use SunLionet, report issues, and participate in discussions.
- Do not need accounts on any single platform to be considered part of the community.

### Contributors

- Submit pull requests, patches, test results, templates, and documentation improvements.
- Do not receive any privileged access by default.

### Reviewers

- Provide code review, testing feedback, and security review in their area of expertise.
- Earn reviewer status through consistent, high-quality review contributions.

### Maintainers

- Merge pull requests, manage releases, and steward the roadmap.
- Operate under strict transparency rules and shared ownership.
- Must not be a single-person bottleneck.

### Security Review Group (SRG)

- A small set of trusted reviewers for sensitive modules.
- Membership is public, time-bounded, and rotation-friendly.
- Provides additional review for changes that affect user safety or trust roots.

## Decision-Making

### Scope Categories

- Small changes: bug fixes, refactors, documentation corrections, non-sensitive improvements.
- Major changes: protocol support, distribution workflow changes, state/storage format changes, UX defaults that affect safety.
- Security-critical changes: cryptography, signature verification, bundle verification, update logic, key handling, logging policies, network transport selection logic.

### Process

- Small changes: maintainer approval after at least one independent review.
- Major changes: public proposal + discussion window + maintainer decision recorded in a decision note (in-repo).
- Security-critical changes: SRG review required + maintainer approval, with explicit risk analysis in the PR description.

### Blocking Rules

- Any maintainer or SRG member can request additional review for safety-sensitive code.
- Any change that increases data exposure risk must be rejected unless the benefit is compelling and mitigations are clear.

## Transparency Rules

- No private roadmaps for security, distribution, or trust roots.
- Prefer public discussions and issues; use private channels only for responsible disclosure.
- All releases have public verification steps and signed artifacts.
- Governance changes are treated as major changes and documented.

## Maintainer Set and Continuity

### Minimum Maintainer Set

- Target: at least 3 active maintainers from different networks/regions.
- If the set drops below 2 active maintainers, release cadence pauses except for critical security fixes.

### Release Authority

- Releases require at least 2 maintainer signatures (2-of-N) before publishing.
- Signing keys are personal and hardware-backed when possible.
- Public key fingerprints are published in-repo and in multiple mirrors.

### Bus-Factor Mitigation

- Every sensitive module must have at least 2 knowledgeable reviewers.
- Maintain “how to release” and “how to verify” steps as first-class docs.
- Avoid hidden operational knowledge (no “only one person knows” steps).

## Module Ownership (Non-Exclusive)

- Ownership provides responsibility for review, not exclusive control.
- Sensitive areas require additional scrutiny:
  - cryptography and signature verification
  - bundle format and verification
  - update and distribution tooling
  - local encrypted storage and key handling
  - logging and diagnostics pathways

## Fork and Mirror Policy

- Forks are encouraged when they preserve user safety and verification.
- Mirrors must not change trust roots silently; if they do, they must clearly document that they are a different trust domain.

## Conflict Resolution

- Technical conflicts: resolve by evidence (tests, reproducible results, threat modeling).
- Governance conflicts: resolve publicly with a recorded decision and rationale.
- Code of conduct: enforce consistently; moderation is separate from technical privilege.
