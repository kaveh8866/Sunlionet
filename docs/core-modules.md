# SunLionet: Core Control Plane Architecture

This document specifies the technical implementation of the Core Control Plane modules for SunLionet, addressing real-time network-interference detection, deterministic policy routing, and secure storage.

## 1. Detector Subsystem
The Detector subsystem provides continuous, stealthy monitoring of network health and censorship events.

### Detection Methods
- **DNS Poisoning Check**: Validates DNS resolution against a small set of known test domains. If responses look like poisoning (e.g., unexpected private IPs or known sinkholes), it flags suspected DNS poisoning.
- **SNI/TLS Reset Detection**: Initiates a TLS handshake to a test SNI. A TCP RST immediately after ClientHello is a strong indicator of active interference.
- **HTTP Injection Detection**: Sends a plain HTTP GET to a known URL. Unexpected 403/redirects or injected HTML indicates middlebox tampering.
- **QUIC/UDP Drop Detection**: Sends a 1-byte UDP ping. If dropped while TCP works, it flags suspected UDP throttling/blocking.
- **Baseline Connectivity**: Periodically checks known reachable endpoints to distinguish between targeted interference and a total outage.

### Operational Tactics
- **Jitter**: All active probes use randomized wait intervals (e.g., `time.Sleep(60 + rand.Intn(60))`) to reduce fingerprinting risk from steady heartbeats.
- **Probe Budget**: Limits active probing (e.g., max 10 TCP/UDP probes per hour) to stay under the radar.
- **Passive Monitoring**: Hooks into sing-box stats and OS network metrics (like TCP retransmissions) to infer health without sending synthetic packets.

## 2. Policy Engine (Deterministic)
The Policy Engine handles 80-90% of routing decisions using hardcoded rules before escalating to the LLM advisor.

### Health Scoring and Ranking
Profiles are evaluated continuously and ranked using the following metrics:
- **`score`**: Composite EWMA (Exponential Weighted Moving Average) score.
- **`success_ewma`**: Ratio of successful connections to total attempts.
- **`median_handshake_ms`**: Latency penalty for slow handshakes.
- **`last_fail_reason`**: e.g., "udp_block_suspected" or "dns_poison_critical".
- **`cooldown_until`**: A UNIX timestamp. Profiles in cooldown are excluded from selection.

### Decision Rules
- **DNS Poisoning**: Switch to a secure DNS tunnel profile immediately. Cooldown current profile for 30 minutes.
- **UDP Blocking**: Switch from UDP-based families (Hysteria2, TUIC) to TCP-based ones (Reality, ShadowTLS).
- **Handshake Bursts**: Switch profile within the same protocol family (assuming the specific endpoint IP/port was blocked).

## 3. Secure Local Store
The Secure Store ensures that all config material and event logs are safely encrypted at rest.

### Storage Architecture
- **Linux/macOS**: Encrypted using `age` (X25519) and AES-GCM.
- **Android**: To be implemented via Android Keystore and SQLCipher in the wrapper application.

### Components
- **Seed Profiles**: Stores all known proxy templates and credentials.
- **Bounded Event Ring Buffer**: Retains only the last 50 events in memory/disk. Older events are overwritten to reduce forensic exposure. Logs do not contain visited domains, only proxy metadata and interference signals.
- **Trusted Contacts**: List of Ed25519 public keys allowed to send `snb://v2` config bundles via Signal.
- **Wipe-on-Suspicion**: The `WipeOnSuspicion()` function overwrites the local database file with random bytes and zeros out the master key in memory when triggered by a panic switch.

## 4. Integration with shadownetd Supervisor (legacy name during transition)
The supervisor loop orchestrates the pipeline:
1. `Detector` emits an event to the `EventChan`.
2. The `Supervisor` appends the event to the ring buffer.
3. The `Policy Engine` evaluates the events and ranks profiles.
4. If a deterministic rule applies, an `ActionSwitchProfile` is generated.
5. If ambiguous, the `LLM Advisor` is invoked with the ranked candidate list.
6. The `Supervisor` generates a new JSON configuration and calls `ApplyAndReload` on the `sbctl` (sing-box controller) for a seamless atomic config reload.
