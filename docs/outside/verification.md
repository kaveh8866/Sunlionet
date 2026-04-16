# Outside: Verification

Verification is designed for:

- operators (manual checks before distribution)
- CI (fail if any trust-critical check fails)
- Inside importer compatibility (same validation logic)

## Verify from a Bundle File

Encrypted bundles (recommended):

```bash
go run -tags outside ./cmd/outside verify \
  --bundle ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt
```

Plaintext bundles:

```bash
go run -tags outside ./cmd/outside verify \
  --bundle ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub
```

Shortcut form (supported):

```bash
go run -tags outside ./cmd/outside --verify ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub
```

## Verify from a URI

```bash
go run -tags outside ./cmd/outside verify \
  --uri-file ./dist/bundle.uri.txt \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt
```

## Machine-Readable Output (CI)

Use JSON output for CI parsing:

```bash
go run -tags outside ./cmd/outside verify \
  --bundle ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt \
  --json
```

## What Verification Checks

Header:

- strict JSON parsing (unknown fields rejected; no trailing JSON)
- `magic`, `bundle_id`, `seq`, timestamps
- expiration and clock-skew sanity
- cipher and encryption metadata consistency
- issuer metadata: `publisher_key_id` must match the trusted signer key id
- signature validity

Payload (requires decryption for encrypted bundles):

- strict JSON parsing and schema version
- deterministic canonical encoding (`payload_bytes == canonical(payload)`)
- issuer metadata presence and consistency (`notes.issuer_key_id`, `profile.source.publisher_key`)
- per-profile validation and normalization rules
- duplicates and trust problems (duplicate IDs and duplicate endpoints)
- template requirements (every profile’s template key must exist; template text must be valid JSON)

