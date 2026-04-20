# Adaptive Intelligence System

SunLionet now includes a local-first adaptive intelligence layer that learns from real connection outcomes and improves profile selection over time without sending sensitive data off-device.

## Safety Model

- Learning state is local-only and encrypted at rest (`adaptive.enc`) with the same 32-byte master key used by profile storage.
- Decisions remain explainable. Every selection has a reason string and scored candidates.
- Deterministic fallback always exists. If adaptive scoring cannot produce a valid candidate (or orchestrator is unavailable), policy ranking is used.
- Adaptive mode is bounded and resettable:
  - Event window is limited to 50-100 records (default 80).
  - Cooldowns are short-lived (default 60 seconds per failed profile).
  - Full reset is available from CLI and mobile bridge.

## Real Signal Inputs

Each attempt records practical runtime signals:

```json
{
  "latency_ms": 320,
  "connect_success": false,
  "dns_ok": false,
  "tcp_handshake": true,
  "tls_success": false
}
```

These are normalized into local `NetworkEvent` records:

```go
type NetworkEvent struct {
    Timestamp int64
    ProfileID string
    Success   bool
    Latency   int
    Failure   string
}
```

Failure normalization maps runtime reasons into bounded categories such as:

- `DNS_BLOCKED`
- `UDP_BLOCKED`
- `TLS_BLOCKED`
- `TIMEOUT`
- `NO_ROUTE`
- `TCP_RESET`
- `UNKNOWN`

## Scoring

Adaptive scoring is O(n) across profiles and uses a rolling, decayed event window:

```text
score = success_rate*100 - latency_ms/10 - failure_weight*5 + trust_adjustments
```

Key behavior:

- Recent events have stronger influence than old events (decay).
- Profiles in cooldown are skipped.
- DNS-block evidence excludes `dns-dependent`/DNS tunnel profiles.
- UDP-block evidence strongly prefers TCP transports.
- TLS-block evidence nudges toward profiles with alternate TLS fingerprints.

## Selection And Diversity

Selection flow:

1. Filter valid profiles (enabled, not manually disabled, not cooling down).
2. Score candidates from local adaptive history.
3. Select highest score.
4. Apply diversity guard when needed.

Diversity guard:

- If the same profile is repeatedly selected 10+ times and score gap is small, choose the next close profile to avoid lock-in.

## Dynamic Fallback

Fallback is not fixed `next profile` order. It adapts to failure signatures:

- DNS failure -> avoid DNS-dependent profiles.
- UDP failure -> prioritize TCP profiles.
- TLS failure -> shift toward different fingerprint/protocol characteristics.

If adaptive filtering becomes too strict and yields no candidates, deterministic fallback ranking is used.

## Optional Koog/Pi Integration (Bounded)

Pi orchestration is only consulted when ambiguity exists:

- close top scores,
- multiple recent failure classes,
- or unknown failure patterns.

When invoked, the request includes bounded adaptive context (`scores`, `recent_failures`) and remains guarded by:

- strict JSON-RPC decoding,
- response validation and safety enforcement,
- timeout and deterministic fallback.

## Transparency

Selection logs include explainable reasons, for example:

```text
[decision] selected=reality-2 reason=score=72.0 highest success_rate with latency/failure penalties
```

Runtime events also expose selected profile, confidence, candidate list, and fallback chain.

## Reset And Controls

CLI reset command:

```bash
sunlionet-inside reset-learning --state-dir <path> --master-key <key>
```

If you are running older releases that still ship legacy binaries, the command may be:

```bash
SUNLIONET-inside reset-learning --state-dir <path> --master-key <key>
```

Runtime toggle:

- CLI: `--adaptive-mode=true|false`
- Mobile bridge config: `adaptive_mode`

Mobile bridge also exposes a `ResetLearning()` function for app-level controls.

## Android Integration

- Adaptive state is encrypted and persisted in `state_dir/adaptive.enc`.
- Selection in the mobile runtime reuses adaptive state for faster reconnection and profile reuse.
- Successful and failed runtime attempts update the same local adaptive history.

## Limitations

- This is heuristic learning, not heavy ML.
- Inference is local and bounded by short history windows.
- Very sparse history may produce conservative behavior until enough attempts are observed.
- If configuration-level errors dominate (missing binary/template), learning can only react to those failure categories.
