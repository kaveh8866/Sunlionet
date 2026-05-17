# Configuration Ingestion Security Audit

Scope: `pkg/importctl/import.go`, `pkg/importctl/replay_store.go`,
`pkg/profile/store.go`, and `pkg/bundle` cryptographic verification helpers.

## Critical

None found after hardening.

## High

- **Replay accepted by ID-only state.** The previous importer tracked only
  `bundle_id`, and committed replay state before profile storage completed.
  An attacker replaying a valid older bundle with a fresh ID could roll back
  local configuration if the sequence was stale but not previously observed.
  A failed store write could also permanently consume a bundle ID.
  Mitigation: replay state now binds `publisher_key_id`, monotonic `seq`, and a
  signed 24-byte nonce, with durable replay commitment after profile storage.

- **Signature transcript lacked explicit domain separation and nonce binding.**
  The signature covered marshaled header bytes and ciphertext, but the header
  had no nonce and the transcript had no SunLionet-specific domain tag. This
  left less margin against cross-protocol misuse and replay of otherwise valid
  ciphertext/header pairs.
  Mitigation: signatures now cover
  `SUNLIONET-BUNDLE-V2 || canonical-header-without-sig || 0x00 || ciphertext`,
  and generated bundles include a mandatory signed nonce.

## Medium

- **Signature and public key lengths were not checked before verification.**
  Empty or malformed keys/signatures did not evaluate to valid, but they were
  not rejected explicitly at the trust boundary and could be misclassified.
  Mitigation: public keys must be 32-byte Ed25519 keys and signatures must be
  64 bytes before verification.

- **Oversized bundle and payload parsing was unbounded.** Import paths and age
  decryption used full reads without repository-level limits, enabling memory
  pressure through untrusted channels such as files, QR-derived payloads, or
  relay messages.
  Mitigation: bundle, ciphertext, decrypted payload, URI, and encrypted store
  reads now have explicit upper bounds.

- **Replay persistence failures were ignored.** A replay store write error was
  previously dropped after successful verification.
  Mitigation: replay persistence is fail-closed and returned to the caller.

## Low

- **Encrypted store writes were non-atomic.** A crash or partial write could
  leave a corrupt profile/replay store.
  Mitigation: stores now write temporary files and rename them into place.

- **Decryption errors exposed lower-level details.** Profile store load errors
  wrapped AES-GCM details.
  Mitigation: profile decryption now returns the stable sentinel error only.

## Residual Risk

The replay policy is intentionally strict: each signer must publish increasing
sequence numbers, and older sequence numbers are rejected once a newer bundle is
committed. Key rotation should use a new Ed25519 key ID or continue the sequence
under the same signing key.
