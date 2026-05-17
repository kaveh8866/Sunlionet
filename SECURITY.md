# Security Policy

SunLionet assumes adversarial transport, hostile local networks, and untrusted distribution media. The system is designed to fail closed: unverified configuration bytes are never applied, revoked signers are rejected offline, and Android routing keeps the VPN interface established during proxy-core failure or network churn.

## Supported Versions

| Version | Supported |
| :--- | :--- |
| 0.3.x-rc1 | Yes |
| < 0.3.0 | No |

## Responsible Disclosure

Do not open public issues for vulnerabilities.

Report security issues to `security@sunlionet.org` with GPG-encrypted details when possible. Include:

- affected commit or release,
- exploit class and preconditions,
- reproduction steps,
- impact assessment,
- whether the issue may identify users or operators.

The Security Response Team acknowledges reports within 24 hours. P0/P1 fixes are targeted within 24-72 hours after confirmation. Public advisories are published only after a fixed release or mitigation path exists. See [Vulnerability Response Lifecycle](docs/governance/vulnerability-response.md).

## Trust Assumptions

- Outside distributors may be compromised individually.
- Local media such as QR, BLE, SMS, files, and Signal messages are untrusted byte carriers.
- The Android network may be actively censored, probed, or forced through interface churn.
- The user device secure preference tree and encrypted local state are trusted after platform unlock.
- Root trust keys are controlled by project maintainers and must be held offline or under threshold controls.

## Cryptographic Constraints

### Bundle Verification

- Signature primitive: Ed25519.
- Public key size: 32 bytes.
- Signature size: 64 bytes.
- Key IDs: `ed25519:` plus first 16 base64url characters of SHA-256(public key).
- Wrapper magic: `SNB1`.
- Max bundle size: `2 MiB`.
- Max decrypted payload size: `1 MiB`.
- Bundle nonce: mandatory and replay-tracked.
- Payload JSON must match canonical encoding after decryption.
- Signatures bind header-with-empty-signature plus ciphertext via the bundle signature domain.

### Offline Revocation

Trust updates use:

- `TrustUpdateMagic = "SNB-TRUST-1"`
- `TrustUpdateDomain = "SUNLIONET-TRUST-UPDATE-V1\x00"`
- `TrustStateDomain = "SUNLIONET-TRUST-STATE-V1\x00"`
- default root threshold `2`
- strict monotonic `Version`
- exact `PrevStateHash`
- max TTL `30 days`

Rollback, stale version, invalid previous hash, and insufficient threshold signatures are security failures.

### Onboarding Links

Accepted forms:

- `sunlionet://config/<base64url-envelope>`
- `SL1:<base64url-envelope>`
- chunked QR wrapper `SLQR1:<index>/<total>:<fragment>`, max 8 fragments

The base64url payload is capped at 300 characters. The signed message is:

```text
"SUNLIONET-ONBOARDING-V1\x00" || unsigned-envelope
```

The envelope is rejected unless the embedded signer public key is already trusted. Activation windows may not exceed 24 hours. Host, SNI, tag, and credential fields are strictly length and character filtered before profile storage.

### Erasure Chunks

Chunk magic is `SNCE`, version `1`, text wrapper `SNBEC/1`. Each chunk has a local SHA-256 checksum and the full payload SHA-256. Reassembly only repairs transport bytes; trust is granted only after the reconstructed bundle passes Ed25519 verification.

### Telemetry

Telemetry is opt-in only and dormant by default. Payloads contain enum counters only. No profile IDs, source IPs, SSIDs, IMEI, timezone, device IDs, route names, free text logs, or precise timestamps are accepted. Batches are time-blurred 24-72 hours and encrypted per transmission with fresh X25519 ephemeral keys, HKDF-SHA256, and ChaCha20-Poly1305.

## Data-Leak Containment Invariants

- The Android VPN routes IPv4 and IPv6 defaults through TUN.
- DNS servers exposed to Android are tunnel-local only: `10.0.0.1` and `fd00:736c:6e::1`.
- During proxy failure or interface churn, `SunlionetVpnService` enters `HOLDING` and keeps route ownership.
- `ProxyCore` transitions engage the kill switch before `HotReload`.
- If all candidates fail, the failover engine enters `beacon` and leaves the kill switch engaged.
- No code may write plaintext directly to ambient sockets from the Android wrapper.

## Secure Coding Rules

- Never persist private keys, decrypted payloads, QR contents, or raw logs outside encrypted stores.
- Never add free-form strings to telemetry payloads.
- Never accept public DNS fallbacks in `VpnService.Builder`.
- Use `context.Context` for all goroutine and runtime lifecycle exits.
- Do not block while holding mutexes.
- Use atomic temp-file writes for encrypted state.
- Validate base64url lengths before decoding large payloads.
- Treat duplicate chunk indices, malformed chunk checksums, and mixed chunk metadata as hostile input.
- Keep pprof loopback-only.

## Extension Review Checklist

For a new transport or proxy backend:

1. It implements `proxycore.ProxyCore`.
2. `Validate` is side-effect free and context-cancelable.
3. `HotReload` assumes the kill switch is engaged and emits no plaintext.
4. `Stop` is idempotent.
5. Health checks are passive or obfuscated.
6. It has tests for timeout, degraded health, restart, and all-candidate failure.
7. It does not import identity stores unless it is explicitly an identity module.
8. It passes `go test ./...` and `go test ./tests/chaos`.
9. Race-sensitive changes pass Linux `go test -race ./tests/chaos`.

## Audit Scope Hints

High-value audit targets:

- `pkg/bundle/verify.go`
- `pkg/bundle/chunk_engine.go`
- `pkg/importctl/import.go`
- `pkg/importctl/revocation.go`
- `pkg/proxycore/failover.go`
- `pkg/mobilebridge/onboarding.go`
- `pkg/mobilebridge/ble_mesh.go`
- `pkg/telemetry/telemetry.go`
- `android/app/src/main/java/com/sunlionet/agent/SunlionetVpnService.kt`
- `android/app/src/main/java/com/sunlionet/agent/Bridge.kt`

## Incident Handling

If a signing key is compromised:

1. Generate a `TrustUpdateBlock` with `revoke_signer`.
2. Sign it with the configured root threshold.
3. Distribute it over all offline media.
4. Verify clients reject bundles from the revoked key.
5. Publish advisory after mitigation is available.

If a VPN leak is suspected:

1. Stop rollout.
2. Reproduce with chaos and Android network-switch tests.
3. Verify TUN route ownership, DNS servers, and `HOLDING` state.
4. Patch before enabling new transport features.
