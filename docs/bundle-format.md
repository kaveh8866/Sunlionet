# Secure Config Bundle Format

ShadowNet uses signed bundles to move seed profiles from Outside → Inside without a central API.

## v1 (current: signed, unencrypted)

Implemented by [import.go](../pkg/importctl/import.go).

Wire format:

```
snb://v1:<base64url(payload_json)>.<base64url(ed25519_signature)>
```

Payload JSON (current):

```json
{
  "ts": 1713100000,
  "profiles": [],
  "revoked_ids": []
}
```

Security properties:

- Integrity/authenticity: Ed25519 signature verified against a trusted public-key list.
- Confidentiality: none (payload is readable to anyone who sees the message).

## v2 (recommended: signed + encrypted payload)

The repository contains a richer bundle schema in [bundle.go](../pkg/bundle/bundle.go) intended for encrypted distribution. v2 should be used for production deployments because it provides confidentiality in addition to authenticity.

Recommended wire format:

```
snb://v2:<base64url(header_json)>.<base64url(ciphertext)>.<base64url(ed25519_signature)>
```

- `header_json` contains authenticated metadata (versioning, key IDs, timestamps, cipher suite).
- `ciphertext` is an encrypted `BundlePayload` JSON.
- `signature` is Ed25519 over `header_json || "." || ciphertext`.

Bundle payload schema:

```json
{
  "schema_version": 1,
  "min_agent_version": "1.0.0",
  "profiles": [],
  "revocations": [],
  "policy_overrides": {
    "cooldown_hard_fail_sec": 900,
    "max_switches_per_10min": 6
  },
  "templates": {},
  "notes": {}
}
```

Cipher suite recommendation:

- X25519 + AEAD (e.g., ChaCha20-Poly1305)
- One-time sender ephemeral key per bundle
- Short expiry (`expires_at`) to reduce replay value

Inside must treat all failures as fail-closed: reject unknown versions, expired bundles, invalid signatures, and decryption failures.

