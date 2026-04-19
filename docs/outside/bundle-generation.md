# Outside: Bundle Generation

SunLionet Outside curates a set of seed profiles, applies strict normalization, embeds the required sing-box outbound templates, then produces a signed bundle (optionally encrypted to an Inside recipient).

Outputs are designed to be safely distributed as:

- a file attachment (`bundle.snb.json`)
- a short text URI (`bundle.uri.txt`) suitable for copying, messaging, or QR encoding
- chunked text lines (`bundle.chunks.txt`) if the URI is too large for typical chat limits

## Key Generation

Generate an Ed25519 signing keypair, and optionally an age identity+recipient pair for encrypting bundles to Inside:

```bash
go run -tags outside ./cmd/outside keygen \
  --ed25519-priv ./keys/outside.ed25519 \
  --ed25519-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --age-recipient ./keys/inside.agepub
```

## Input Profiles

Provide candidate profiles via one of:

- `--profiles ./profiles.json` (single object or array)
- `--profiles-dir ./profiles/` (directory of `*.json` files)

Unknown JSON fields are rejected. Profiles are normalized and validated before selection.

## Generate a Bundle

Encrypted bundle (recommended):

```bash
go run -tags outside ./cmd/outside \
  --profiles ./profiles.json \
  --templates-dir ./templates \
  --signing-key ./keys/outside.ed25519 \
  --recipient-pub ./keys/inside.agepub \
  --out ./dist
```

Signed plaintext bundle (only when you explicitly allow it):

```bash
go run -tags outside ./cmd/outside \
  --profiles ./profiles.json \
  --templates-dir ./templates \
  --signing-key ./keys/outside.ed25519 \
  --allow-plaintext \
  --out ./dist
```

## Outputs

`--out ./dist` produces:

- `bundle.snb.json` (signed wrapper; ciphertext is base64url)
- `bundle.uri.txt` (`snb://v2:<base64url(wrapper_json)>`)
- `manifest.json` (machine-readable distribution manifest)
- `summary.txt` (operator-oriented summary)
- `issuer_pub.b64url` (signer public key for distribution/verification)
- `bundle.sig.b64url` (signature string from header)

Optional, depending on size:

- `qr_payload.txt` (written only if the URI length is <= `--qr-threshold-chars`)
- `bundle.chunks.txt` and `bundle.chunks.json` (written only if URI length exceeds `--chunk-threshold-chars`)

## Chunked Transport Preparation

If the bundle URI is too large for a messaging channel, Outside can prepare a chunked text representation:

- `bundle.chunks.txt` contains `SNBCHUNK/1 <id> <i>/<n> <data>` lines
- the receiver can reconstruct the original URI by concatenating the chunk data in order

This is preparation only; SunLionet does not automatically send via third-party messengers yet.
