# Field Testing Plan (Telemetry-Free, Safety-First)

## Principles

- No telemetry, no background uploads, no identifiers.
- Assume hostile environment: DPI, inspection, blocked platforms.
- Protect testers: minimize exposure, clear panic procedures.
- Prefer reproducible scenarios over raw logs.

## Staged Rollout

### Stage 1: Controlled Testing (Trusted Group)

Scope:

- 5–20 trusted testers.
- Known devices (at least one low-end device).
- Known bundles from a small set of publishers.

Goals:

- Validate install/import/connect/disconnect.
- Validate that failures are explicit and recoverable.
- Validate that default settings do not degrade normal connectivity silently.

Feedback:

- Manual reports using `report.json` + short checklist.
- No screenshots by default.

### Stage 2: Semi-Open Testing (Limited Public)

Scope:

- Testers self-select from trusted communities.
- Distribution via mirrors + peer transfer.

Goals:

- Validate survival across diverse networks and device settings.
- Validate update flow (offline APK + bundle updates).

Feedback:

- Optional manual report export.
- Structured issue templates (copy/paste) for non-technical users.

### Stage 3: Open Deployment

Scope:

- Public availability with decentralized distribution.

Goals:

- Improve robustness, docs, and compatibility based on aggregated patterns.
- Keep release process strict (signed manifests, reproducible builds where possible).

Feedback:

- Community-run “testing scenarios” repo: testers share sanitized outcomes only.

## Test Scenarios

### Normal Filtered Internet

- Install APK and import bundle.
- Connect and keep VPN active for 10 minutes.
- Switch airplane mode on/off once while connected.

### Heavy DPI Filtering

- Attempt connect three times with 30s gaps.
- Record which protocols succeed (family-level only).

### Protocol Blocking

- UDP likely blocked: verify TCP-only profiles succeed.
- TCP selective blocking: verify alternative transports survive.

### Intermittent Connectivity

- Walk between networks (Wi-Fi ↔ mobile data).
- Observe whether the app recovers without loops.

### Full Blackout / Offline Mode

- Verify app starts and shows state without network.
- Verify bundle import works from local storage.

### Device Constraints

- Battery saver enabled.
- Low RAM device or background-kill stress.
- Verify services remain foreground and recover cleanly.

## Objectives and What to Record

- Connection outcome: success/failure/partial.
- Time-to-connect bucket: <10s / <30s / >30s.
- VPN stability: drops per 30 min (approx).
- Bundle import: success/failure + reason category.
- Switching behavior: none / occasional / excessive (subjective).
- Battery impact (manual): “low/medium/high” perception over 30–60 min.
- Crash frequency: none / once / repeated.

## Safety Procedures

- Prefer neutral app naming and minimal logs in high-risk contexts.
- Do not share raw configs, endpoints, or profile IDs.
- In panic situations: disconnect first, then clear app data if needed.

