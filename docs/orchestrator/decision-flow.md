# Decision Flow (Detector → Policy → Pi → Executor)

## Primary Path

1. Detector produces events and health signals.
2. Policy ranks candidates deterministically and emits a primary selection + confidence estimate.
3. If ambiguity is detected (or `--use-pi` is set), Inside sends a strict JSON-RPC request to Pi with:
   - network state (if available)
   - candidate profile snapshots
   - recent decision history
   - constraints (e.g., no UDP)
4. Pi returns an explicit action + reason + confidence.
5. Inside validates Pi output strictly and enforces safety constraints.
6. Executor applies the final decision (render config → validate/apply to sing-box).

## Fallback Path (Fail-Safe)

If any of the following occurs:

- Pi cannot be started / connected
- Pi times out
- Pi returns invalid JSON
- Pi returns unknown fields/actions
- Pi suggests an invalid/unsafe profile

Then Inside logs the failure reason and proceeds with the deterministic Policy selection.

## Logging (Auditability)

Inside logs:

- `[policy] candidates=... policy_confidence=...`
- `[orchestrator] invoked`
- `[orchestrator] decision=... profile=... confidence=... reason=...`
- fallback triggers (unavailable/timeout/invalid/rejected)

