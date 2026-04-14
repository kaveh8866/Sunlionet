# ShadowNet Agent (Inside + Outside)

ShadowNet is an offline-first censorship-circumvention agent designed for high-risk DPI environments (e.g., Iran). The project builds two tightly coordinated binaries from the same codebase:

- ShadowNet-Inside (Iran): runs on end-user devices inside Iran and keeps connectivity alive by detecting interference and rotating local sing-box configurations.
- ShadowNet-Outside (Exile/Helper): runs outside Iran and acts as a config factory to generate, validate, and distribute signed/encrypted seed bundles to Inside users (primarily via Signal).

Documentation site (GitHub Pages): https://kaveh8866.github.io/shadownet-agent/ (enable Pages to serve from `/docs` on `main`).

Persian: see [README.fa.md](README.fa.md).

---

## Quick Start (Development)

Prerequisites:

- Go 1.21+
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

---

## Build

This repository produces two binaries using Go build tags.

### ShadowNet-Inside (Iran)

```bash
go build -tags inside -ldflags="-s -w" -o bin/shadownet-inside ./cmd/inside/
```

```bash
./bin/shadownet-inside
```

### ShadowNet-Outside (Exile/Helper)

```bash
go build -tags outside -ldflags="-s -w" -o bin/shadownet-outside ./cmd/outside/
```

```bash
./bin/shadownet-outside
```

---

## Config Distribution (Outside → Inside)

ShadowNet intentionally avoids a central fetch API. Seed configs arrive via:

1. Signal disappearing messages: signed/encrypted `snb://v1:` bundles.
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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
