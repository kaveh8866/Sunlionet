# Linux MVP Install + Run

This guide covers the real Linux MVP runtime path for `cmd/inside`:

1. Initialize state
2. Import signed/encrypted bundle from disk
3. Rank/select profile with deterministic policy
4. Render sing-box config
5. Validate and launch sing-box (or fail with actionable error)

## Prerequisites

- Go `1.25+`
- Optional: `sing-box` in `PATH`, or pass `--sing-box-bin`

## Required inputs

- Bundle file: `./testdata/sample.bundle`
- Trusted signer public key (base64url): `./testdata/sample_signer_pub.b64url`
- Age identity (X25519): `./testdata/sample_age_identity.txt`
- 32-byte local master key

## Full run (render + execute)

```bash
go run ./cmd/inside \
  --state-dir ./.tmp/state \
  --import ./testdata/sample.bundle \
  --master-key 0123456789abcdef0123456789abcdef \
  --trusted-signer-pub-b64url "$(cat ./testdata/sample_signer_pub.b64url)" \
  --age-identity "$(cat ./testdata/sample_age_identity.txt)"
```

## Render only

```bash
go run ./cmd/inside \
  --state-dir ./.tmp/state \
  --import ./testdata/sample.bundle \
  --master-key 0123456789abcdef0123456789abcdef \
  --trusted-signer-pub-b64url "$(cat ./testdata/sample_signer_pub.b64url)" \
  --age-identity "$(cat ./testdata/sample_age_identity.txt)" \
  --render-only
```

## Validate only

```bash
go run ./cmd/inside \
  --state-dir ./.tmp/state \
  --import ./testdata/sample.bundle \
  --master-key 0123456789abcdef0123456789abcdef \
  --trusted-signer-pub-b64url "$(cat ./testdata/sample_signer_pub.b64url)" \
  --age-identity "$(cat ./testdata/sample_age_identity.txt)" \
  --validate-only
```

## State and outputs

- Profiles store: `<state-dir>/profiles.enc`
- Templates store: `<state-dir>/templates.enc`
- Runtime config: `<state-dir>/runtime/config.json`
- Runtime state: `<state-dir>/state.json`

The persisted state includes:

- selected profile id
- selection reason
- fallback candidates
- rendered config path
- sing-box binary and PID

## Common errors

- `missing or invalid master key`: set `--master-key` to exactly 32 bytes.
- `missing trusted signer keys`: set `--trusted-signer-pub-b64url`.
- `missing age identity`: set `--age-identity`.
- `sing-box binary not found`: set `--sing-box-bin` or install `sing-box`.

## Current limitations

- Linux-first runtime path is prioritized.
- Android production packaging is out of scope for this MVP.
- LLM advisor is not required for startup path; deterministic policy ranking is used for selection.
