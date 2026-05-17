# SunLionet Chaos Testing Strategy Matrix

All chaos tests are deterministic and seed-based. The default seed is
`0x51A7E20260517`, so a failing run can be reproduced locally with the same
test command. The suite is compatible with Go's race detector:

```text
go test -race ./tests/chaos
```

| Phase | Test | Injected Vector | Expected Self-Healing | Validation Boundary |
| --- | --- | --- | --- | --- |
| Packet loss and jitter | `TestChaosExtremeLossJitterChunkReassembly` | 85% deterministic frame loss plus jitter during erasure-coded chunk delivery | Any `N` surviving chunks reconstruct the exact original bundle bytes | SHA-256 verified payload equality; no chunk parser panic |
| Interface flapping | `TestChaosRapidInterfaceFlappingFailClosedNoDeadlock` | 24 concurrent failover switches simulating broken Wi-Fi/cellular/offline churn | Engine settles into beacon/isolated state and keeps kill switch engaged | Test timeout catches deadlocks; mock VIF records zero plaintext bytes |
| Malicious chunks | `TestChaosMaliciousChunksRejectedAndCacheBounded` | Corrupted checksum, oversized shard body, cache overflow | Reassembler rejects malformed chunks and drops over-budget partials | Errors must be typed checksum/cache-limit failures |
| Reconnect storm | `TestChaosMemoryStableAcrossReconnectStorm` | 500 repeated degraded failover attempts with blocked health samples | No runaway allocation and kill switch remains engaged | Heap growth bounded after GC; no panic |
| Diagnostics privacy | `TestChaosTelemetryEnumsOnlyUnderFailure` | Failure events for reset, forged signature, and BLE interruption | Telemetry queue stores enum-only counters under opt-in config | Queue contains no profile/identity/log/free-text tokens |

## CI Guidance

Fast presubmit:

```text
go test ./tests/chaos
```

Nightly or release gate:

```text
go test -race ./tests/chaos
go test -run TestChaosMemoryStableAcrossReconnectStorm ./tests/chaos
```

The chaos suite intentionally uses mocks for the virtual interface and proxy
core process. It links directly to the production failover, telemetry, and
erasure reassembly packages, but avoids requiring root, Android emulators, or
host firewall mutation in CI.
