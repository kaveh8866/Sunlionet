# SunLionet: Core Control Plane Architecture

This document maps core control-plane modules to the current codebase. Where a module is not implemented (or exists only in a non-release build), it is labeled explicitly.

## 1. Detector Subsystem
The Detector subsystem provides monitoring of network health and censorship signals.

### Detection Methods
- Implemented in the release build (`sunlionet-inside`):
  - Baseline TCP connectivity probe + latency measurement.
  - DNS poisoning suspicion check.
  - UDP reachability check (coarse “UDP blocked” signal).
  - Code: `pkg/detector/real/real.go` and `pkg/detector/*`.
- Not implemented in the release build (conceptual/planned):
  - SNI/TLS reset detection.
  - HTTP injection detection.
  - QUIC-specific detection.

## 2. Policy Engine (Deterministic)
The Policy Engine ranks candidates deterministically and produces a safe fallback decision path when optional orchestrators are unavailable.

### Health Scoring and Ranking
Implemented in the release build:

- Ranking and filtering logic: `pkg/policy/engine.go` (EWMA success, latency penalties/bonuses, cooldown filtering, trust weighting).
- Optional adaptive learning and diversity guards: `pkg/policy/adaptive.go` + `pkg/policy/adaptive_store.go` (encrypted at-rest).
- Optional external “orchestrator” integration (Pi): `pkg/orchestrator/*` + `cmd/inside` flags. Orchestrator output is enforced by safety constraints and falls back to deterministic policy on failure.

LLM-based decisioning exists in a non-release runtime path (`cmd/inside` with build tag `daemon`) and is not part of the current public release boundary.

## 3. Secure Local Store
The Secure Store ensures that configuration state is encrypted at rest on disk. Runtime event streams are separate and are intentionally bounded/sanitized.

### Storage Architecture
- **Go (Inside/Outside)**: Encrypted JSON storage using AES-256-GCM with a 32-byte master key (see `pkg/profile/store.go` and other `pkg/*/store.go`).
- **Android**: Keystore-backed encrypted preferences for app secrets/state (see `android/app/src/main/java/com/sunlionet/agent/SecureStore.kt`).

### Components
- Seed profiles and templates (encrypted at rest): `pkg/profile/*`.
- Trusted signer allowlist (import boundary): `pkg/importctl/import.go` and `pkg/bundle/verify.go`.
- Wipe-on-suspicion behavior (current): `pkg/profile/store.go` deletes the encrypted store file on request.

## 4. Integration with SUNLIONETd Supervisor (legacy name during transition)
Conceptual (non-release): the supervisor/daemon architecture described below reflects an earlier or experimental control loop (see `cmd/inside/daemon.go`) and is not the default release runtime.

The supervisor loop orchestrates the pipeline:
1. `Detector` emits an event to the `EventChan`.
2. The `Supervisor` appends the event to the ring buffer.
3. The `Policy Engine` evaluates the events and ranks profiles.
4. If a deterministic rule applies, an `ActionSwitchProfile` is generated.
5. If ambiguous, the `LLM Advisor` is invoked with the ranked candidate list.
6. The `Supervisor` generates a new JSON configuration and calls `ApplyAndReload` on the `sbctl` (sing-box controller) for a seamless atomic config reload.
