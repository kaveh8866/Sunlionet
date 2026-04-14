# Secure Config Bundle Format

ShadowNet uses signed bundles to move seed profiles from Outside → Inside without a central API.

## v2 (signed + encrypted payload)

Implemented by [crypto.go](../pkg/bundle/crypto.go) and consumed by [import.go](../pkg/importctl/import.go).

Wire format:

```
snb://v2:<base64url(wrapper_json)>
```

Where `wrapper_json` is:

```json
{
  "header": {
    "magic": "SNB1",
    "bundle_id": "bndl_...",
    "publisher_key_id": "key-1",
    "recipient_key_id": "default",
    "seq": 1,
    "created_at": 1713100000,
    "expires_at": 1713700000,
    "cipher": "age-x25519",
    "signature": "base64url(ed25519_signature)"
  },
  "ciphertext": "base64url(age_encrypted_payload_bytes)"
}
```

Signature input:

- `ed25519_signature = Sign( MarshalJSON(header_without_signature) || ciphertext_bytes )`

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

- age X25519 recipient encryption
- Short expiry (`expires_at`) to reduce replay value

