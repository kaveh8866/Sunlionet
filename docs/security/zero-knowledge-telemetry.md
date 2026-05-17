# Zero-Knowledge Diagnostic Telemetry

SunLionet telemetry is dormant by default. The Go core only initializes the
diagnostic engine when `telemetry_enabled=true` is provided from the secure
preference tree. Without that explicit opt-in, calls to the telemetry API are
no-ops and no queue file is created.

## Data Schema

Reports are schema-bounded and enum-only. No free-text strings, log lines,
profile IDs, phone IDs, device IDs, SSIDs, timezone values, source IPs, routes,
or precise timestamps are accepted by the Go telemetry API.

Each local event is stored as:

| Field | Type | Allowed Values |
| --- | --- | --- |
| `c` | `uint16` | Diagnostic enum such as `EventDNSBlocked`, `EventTCPReset`, `EventImportSignatureInvalid` |
| `v` | `uint16` | Coarse core version enum, currently `CoreVersionSunLionetV1` |
| `k` | `uint8` | Coarse carrier class enum: unknown, simulated, mobile, wifi, mixed |

The encrypted report contains aggregate counters only:

```text
schema:uint8 || transport:uint8 || counter_count:uint16 ||
  repeated(counter.event:uint16, counter.core:uint16, counter.carrier:uint8, noisy_count:uint16)
```

## Time Blurring

The engine never emits a packet at the moment an error happens. On first event,
it schedules `next_after` randomly between 24 and 72 hours in the future. Events
are batched and shuffled into aggregate counters. A delivery failure is silent.
The queue is capped at 50 KiB and self-destructs if that limit is reached, so
telemetry cannot become a durable activity trail.

## Differential Privacy

Each aggregate counter receives small symmetric discrete noise:

```text
P(noise=-1) = 1/16
P(noise=+1) = 1/16
P(noise=0)  = 14/16
```

Counts are clamped at zero. This is intentionally lightweight local noise: it
reduces exact single-user contribution leakage while preserving coarse failure
trends when many opt-in clients contribute.

## Cryptographic Blinding

Every transmission uses a new X25519 ephemeral key:

```text
shared = X25519(ephemeral_private, collector_public)
key    = HKDF-SHA256(shared, random_32_byte_salt, "SUNLIONET-TELEMETRY-V1")
ct     = ChaCha20-Poly1305(key).Seal(report, aad=(transport, schema))
```

The envelope contains only schema, transport enum, ephemeral public key, random
salt+nonce, and ciphertext. Repeated identical reports produce unrelated
ciphertexts.

## Network Guardrails

The production engine rejects direct telemetry endpoints. Delivery must be
configured for an approved privacy transport: Tor `.onion`, I2P `.i2p`, mixnet,
or domain-fronted relay. Test-only direct delivery requires an explicit
`AllowUnsafeDirectForTests` flag that is not wired from Android preferences.

The endpoint may observe the relay/mix exit, but not the user's true source IP
when deployed behind the required privacy transport. The report itself is
encrypted to the collector public key and cannot be inspected by intermediate
relays.

## Integration

Runtime and import hooks record only coarse enums:

- config invalid
- proxy handshake timeout
- core start failure
- import signature invalid
- import replay detected
- unknown failure

No user configuration store or identity cache is imported by `pkg/telemetry`.
