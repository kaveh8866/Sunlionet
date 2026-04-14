# Threat Model (Dual-Agent Setup)

This document focuses on the dual-version model: ShadowNet-Inside operates in a high-risk, hostile environment; ShadowNet-Outside operates in a safer jurisdiction.

## Assumptions

- Inside devices can be seized and inspected.
- Censors can monitor and interfere with network traffic (DPI, resets, blocking, throttling).
- Signal may be throttled or blocked periodically.
- Outside infrastructure and helpers are more protected, but still must assume opportunistic attackers.

## Key goals

- Prevent config injection by untrusted parties.
- Minimize data on Inside devices (least persistence, encrypted at rest).
- Avoid outbound telemetry by default (Inside → Outside).
- Ensure connectivity recovery without user intervention when possible.

## Threats and mitigations

- Device seizure (Inside)
  - Encrypted local store: [store.go](../pkg/profile/store.go)
  - Minimal logging; avoid writing sensitive events to disk

- Malicious config injection
  - Fail-closed signature verification: [import.go](../pkg/importctl/import.go)
  - Trusted public keys configured out-of-band

- LLM misuse / data leakage
  - LLM sees no secrets or full configs
  - Strict JSON-only bounded output and deterministic policy fallback

- Signal compromise / metadata risk
  - Prefer disappearing messages
  - Keep message contents minimal (bundle URI only)
  - v2 encrypted bundles recommended for confidentiality

- Blackout scenarios
  - Local Bluetooth mesh for peer-to-peer seed sharing

