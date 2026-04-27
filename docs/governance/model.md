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
- Emergency changes: hotfixes for P0/P1 incidents, urgent mirror updates during blocking events.

### Process
- Small changes: maintainer approval after at least one independent review.
- Major changes: public proposal + discussion window + maintainer decision recorded in a decision note (in-repo).
- Security-critical changes: SRG review required + maintainer approval, with explicit risk analysis in the PR description.
- Emergency changes: Single maintainer "Emergency Authority" (documented below).

### Emergency Response Authority
In the event of a P0/P1 incident (as defined in `docs/release/incident-response.md`):
1. Any active maintainer can trigger an "Emergency State."
2. During an Emergency State, the 2-of-N signature requirement for hotfixes is waived for a 24-hour window to allow rapid mitigation.
3. The acting maintainer must publish a "Post-Mortem" within 72 hours, justifying the emergency actions.
4. Emergency authority is limited to the scope of the incident (e.g., fixing the specific bug, not adding features).

### Release Decision Ownership
- **Stable Releases**: Unanimous agreement among active maintainers.
- **Beta Releases**: Majority agreement among active maintainers.
- **Hotfixes**: Decision by any maintainer in the Security Response Team (SRT).

### Conflict Resolution
- Technical disputes are resolved through consensus. If consensus cannot be reached after 7 days, the SRG provides a binding safety assessment.
- Governance disputes require a majority vote of all maintainers.

- Technical conflicts: resolve by evidence (tests, reproducible results, threat modeling).
- Governance conflicts: resolve publicly with a recorded decision and rationale.
- Code of conduct: enforce consistently; moderation is separate from technical privilege.
