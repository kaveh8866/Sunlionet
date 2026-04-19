# sing-box Profiles & Outbound Strategy

SunLionet treats outbounds as **pre-validated profiles** (templates + metadata). The policy engine selects a profile and an allowed mutation set; the LLM advisor may recommend a selection when deterministic logic is insufficient.

## Profile data model

Implemented in [profile.go](../pkg/profile/profile.go):

- `id`: stable identifier
- `family`: `reality`, `hysteria2`, `tuic`, `shadowtls`, `dns_tunnel`
- `template_ref`: reference to a vetted config snippet/template
- `endpoint`: host/port/ip-version
- `capabilities`: transport and DPI resistance tags
- `mutation_policy`: allowed mutation sets and rate limiting
- `health`: local EWMA score and cooldown state

## Protocol families

### Reality (primary TCP stealth)

Prefer when:

- UDP/QUIC is degraded or blocked
- TLS camouflage is required (SNI/ALPN/uTLS diversity)

Safe mutation parameters (must be pre-declared in mutation sets):

- SNI choice (from a curated list)
- uTLS fingerprint (from curated list)
- port selection (from allowed ports per server)
- Reality short ID rotation (if supported by the server)

### Hysteria2 (primary UDP performance)

Prefer when:

- UDP is available and stable
- user needs throughput/latency performance

Safe mutation parameters:

- obfs/password rotation (only if pre-shared and bundled safely)
- port hopping (allowed list)
- pacing/jitter knobs (bounded ranges)

### TUIC v5 (secondary UDP/QUIC)

Prefer when:

- Hysteria2 is blocked or unstable
- QUIC-style obfuscation helps with current DPI behavior

Safe mutation parameters:

- congestion control preset (curated)
- port rotation (allowed list)
- SNI/ALPN knobs if tunneled over TLS (template-dependent)

### ShadowTLS v3 (secondary TCP camouflage)

Prefer when:

- TLS impersonation is more effective than Reality
- specific networks aggressively reset SNI patterns

Safe mutation parameters:

- SNI (curated)
- uTLS fingerprint (curated)
- padding/fragmentation knobs (bounded)

### DNS tunnel fallback (last resort)

Prefer when:

- both TCP and UDP are strongly blocked
- only DNS-like egress survives

Safe mutation parameters:

- resolver selection (curated)
- QNAME length/jitter bounds
- retry/cooldown policy

## Local ranking and cooldown

Inside keeps a local health score per profile:

- successes raise `SuccessEWMA` and `Score`
- failures lower `Score` and set `CooldownUntil`
- `LastFailReason` and event types influence deterministic policy routing (e.g., SNI resets vs UDP blackhole)

The policy engine must avoid repeatedly selecting recently failing profiles, and should diversify families when failure reasons suggest systematic blocking.

