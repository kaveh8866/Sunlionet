# Offline Trust-State Machine

SunLionet clients operate without reliable access to online CRLs or OCSP. Trust
updates therefore travel through the same offline channels as configuration
bundles: Signal, BLE, QR, local files, or peer-to-peer sharing.

## Trust Anchors

Clients are bootstrapped with one or more Ed25519 Outside signer public keys and,
optionally, a cold-storage maintainer root set. The root set authorizes changes
to the local trusted signer registry. The registry is stored encrypted at
`state_dir/trust.enc`.

## Revocation Block

A trust update block has:

- `magic`: `SNB-TRUST-1`
- `block_id`: stable identifier for transit and diagnostics
- `version`: strictly increasing registry version
- `prev_state_hash`: hash of the exact previous local trust state
- `issued_at`, `effective_at`, `expires_at`: bounded validity window
- `threshold`: required root signatures, normally 2-of-3
- `operations`: deterministic actions such as `revoke_signer`, `add_signer`,
  `retire_signer`, `add_root`, `retire_root`
- `signatures`: Ed25519 signatures by root keys

The signed transcript is:

```text
SUNLIONET-TRUST-UPDATE-V1 || canonical-update-without-signatures
```

## State Sealing

Each applied state is sealed as:

```text
state_hash = SHA256(SUNLIONET-TRUST-STATE-V1 || canonical-state-without-state_hash)
```

A new block must have `version > current.version` and
`prev_state_hash == current.state_hash`. This prevents rollback and forked
offline histories from silently replacing newer trust state.

## Revocation Semantics

`revoke_signer` removes an Outside signer from the active set and places its key
ID in a compact local revocation table. Incoming bundles are rejected before
decryption if `header.publisher_key_id` is revoked.

`retire_signer` is used for normal rotation. It removes the signer from future
active publication but does not mark it revoked, so already issued bundles remain
usable until their own signed `expires_at`.

## Transit

Trust update blocks are regular JSON byte payloads. They can be copied through
QR, BLE, Signal, or local files. Any updated client can relay the exact block to
nearby offline clients. Because every block contains the previous state hash and
root threshold signatures, transit nodes do not need to be trusted.

## Memory Bound

The local state stores short Ed25519 key IDs, base64 public keys, and compact
revocation/transition records only. Update and state payloads are capped at
256 KiB, suitable for mobile storage and bounded parsing.
