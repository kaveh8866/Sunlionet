# ShadowNet Agent

ShadowNet is an offline-first, bundle-based censorship-resilience system designed for high-risk DPI environments (e.g., Iran).

It ships as:

- ShadowNet Inside: runs on end-user devices in censored networks and keeps connectivity alive by detecting interference and rotating local sing-box configurations.
- ShadowNet Outside: runs by supporters to generate, validate, and distribute signed/encrypted seed bundles to Inside users (primarily via Signal).
- ShadowNet App: the Android wrapper for ShadowNet Inside (VPN integration + bundle import UI).
- ShadowNet Dashboard: the web UI for operator visibility (including local runtime status via localhost-only APIs).

Documentation site (GitHub Pages): https://kaveh8866.github.io/shadownet-agent/ (enable Pages to serve from `/docs` on `main`).

Persian: see [README.fa.md](README.fa.md).

---

## Getting Started (MVP)

Start here: [docs/getting-started.md](docs/getting-started.md).

It covers the three first-time paths:

- Linux: Download → Verify → Install → Import bundle → Connect
- Android: Install APK → Import bundle → Connect → Monitor
- Web: Open ShadowNet Dashboard → Detect local runtime → Show status

## Quick Start (Development)

Prerequisites:

- Go 1.25+
- Windows: PowerShell 7+

Run tests:

- Windows:

  ```powershell
  .\scripts\run_tests.ps1
  ```

- Linux/macOS:

  ```bash
  ./scripts/run_tests.sh
  ```

## Linux MVP (Inside)

Prerequisites:

- Go 1.25+
- Optional: `sing-box` installed, or pass `--sing-box-bin`

Run the Linux MVP path (import -> select -> render -> execute):

```bash
go run ./cmd/inside \
  --state-dir ./.tmp/state \
  --import ./testdata/sample.bundle \
  --master-key 0123456789abcdef0123456789abcdef \
  --trusted-signer-pub-b64url "$(cat ./testdata/sample_signer_pub.b64url)" \
  --age-identity "$(cat ./testdata/sample_age_identity.txt)"
```

Render only (no process launch):

```bash
go run ./cmd/inside \
  --state-dir ./.tmp/state \
  --import ./testdata/sample.bundle \
  --master-key 0123456789abcdef0123456789abcdef \
  --trusted-signer-pub-b64url "$(cat ./testdata/sample_signer_pub.b64url)" \
  --age-identity "$(cat ./testdata/sample_age_identity.txt)" \
  --render-only
```

---

## Build

This repository produces two binaries using Go build tags.

### ShadowNet Inside

```bash
go build -tags inside -ldflags="-s -w" -o bin/shadownet-inside ./cmd/inside/
```

```bash
./bin/shadownet-inside
```

### ShadowNet Outside

```bash
go build -tags outside -ldflags="-s -w" -o bin/shadownet-outside ./cmd/outside/
```

```bash
./bin/shadownet-outside
```

---

## Config Distribution (Outside → Inside)

ShadowNet intentionally avoids a central fetch API. Seed configs arrive via:

1. Signal disappearing messages: signed/encrypted `snb://v2:` bundles.
2. QR codes (in-person bootstrap).
3. Local Bluetooth mesh (Inside ↔ Inside) for blackout scenarios.

Bundle parsing/validation lives in [pkg/importctl](pkg/importctl) and [pkg/bundle](pkg/bundle).

---

## Security Model (Summary)

Inside devices can be seized. Design goals:

- Seed profiles stored encrypted at rest via [store.go](pkg/profile/store.go).
- Config bundles authenticated with Ed25519 signatures via [import.go](pkg/importctl/import.go).
- Default is receive-only from Inside (no outbound telemetry).

For the detailed threat model and dual-agent design, see `/docs`.

## Documentation

- [Core Modules Architecture](docs/core-modules.md) (Detector, Policy Engine, Secure Store)
- [Website & Docs Specification](docs/website-spec.md)
- [Architecture Details](docs/architecture.md)
- [Bundle Format](docs/bundle-format.md)
- [Signal Integration](docs/signal.md)
- [Installation Guide](docs/install.md) (English)
- [Linux MVP Install + Run](docs/install/linux-mvp.md)
- [Linux Smoke Test](docs/dev/linux-smoke-test.md)
- [راهنمای نصب (Persian)](docs/fa/install.md)

## ShadowNet Outside (MVP usage)

Generate keys:

```bash
go run -tags outside ./cmd/outside keygen \
  --ed25519-priv ./keys/outside.ed25519 \
  --ed25519-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --age-recipient ./keys/inside.agepub
```

Generate a bundle:

```bash
go run -tags outside ./cmd/outside \
  --profiles ./profiles.json \
  --templates-dir ./templates \
  --signing-key ./keys/outside.ed25519 \
  --recipient-pub ./keys/inside.agepub \
  --out ./dist
```

Verify a bundle:

```bash
go run -tags outside ./cmd/outside verify \
  --bundle ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt
```

Verify directly from URI text:

```bash
go run -tags outside ./cmd/outside verify \
  --uri-file ./dist/bundle.uri.txt \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt
```

Distribution helpers:

- `./dist/manifest.json` is machine-readable (includes bundle SHA-256 and URI size hints)
- `./dist/qr_payload.txt` is written only if the URI is small enough (`--qr-threshold-chars`)
- `./dist/bundle.chunks.txt` is written only if the URI exceeds `--chunk-threshold-chars` (SNBCHUNK/1 lines)

See:

- [Bundle Generation](docs/outside/bundle-generation.md)
- [Trust Model](docs/outside/trust-model.md)
- [Verification](docs/outside/verification.md)

## problems_and_diagnostics

- “missing --signing-key”: provide `--signing-key` (Ed25519 seed base64url) or run `keygen`.
- “refusing to generate unencrypted bundle…”: pass `--allow-plaintext` or set `--recipient-pub` for encryption.
- “missing outbound template…”: ensure `--templates-dir` points at `./templates` and your profiles’ `template_ref` matches an existing `*.json` template.
- “SECURITY ALERT: signature verification…” (Inside import): you’re using the wrong trusted signer pub key, or the bundle/URI was corrupted.
- “note: header verified; payload not decrypted…” (Outside verify): provide `--age-identity`, and use `--require-decrypt` in CI to fail closed.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
