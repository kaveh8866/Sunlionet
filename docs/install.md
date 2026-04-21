# Installation Guide

This repository currently targets:

- SunLionet Inside: Linux + Android (signed APK and Termux CLI path)
- SunLionet Outside: any OS with stable internet (Linux/macOS/Windows)

Migration note: this repository is still named `SUNLIONET-agent` during the transition. Some internal identifiers and older release artifacts may still use legacy `SUNLIONET-*` naming.

If you are installing for the first time, start here:

- [Getting Started](getting-started.md)

## Build tags

- Inside: `-tags inside`
- Outside: `-tags outside`

## Linux MVP quick path

For the full Linux-first MVP flow (bundle import, profile selection, config render, sing-box validation/launch), use:

- [Linux MVP Install + Run](install/linux-mvp.md)
- [Linux Install Paths](install/linux.md)
- [Artifact Verification](install/verification.md)
- [Linux Smoke Test](dev/linux-smoke-test.md)

## Android install path

- [Android Install (APK)](install/android.md)

## Development prerequisites

- Go 1.25+

This repository pins its Go toolchain in [go.mod](file:///C:/Users/Kaveh/Desktop/Iran-Agent-Vpn/go.mod) via `toolchain`. If your local Go supports toolchains, `go` will auto-download the pinned version.

Optional tools:

- `sing-box` (runtime for real connectivity)
- a local LLM runtime (e.g., llama.cpp) is only required for the non-release daemon build (`inside && daemon`); the public release path does not require an LLM runtime

## Build

```bash
mkdir -p bin

go build -tags inside -ldflags="-s -w -X main.version=v0.1.0" -o bin/sunlionet-inside ./cmd/inside/
go build -tags outside -ldflags="-s -w -X main.version=v0.1.0" -o bin/sunlionet-outside ./cmd/outside/
```

## Run tests

- Windows:

  ```powershell
  .\scripts\run_tests.ps1
  ```

- Linux/macOS:

  ```bash
  ./scripts/run_tests.sh
  ```

## First seed bootstrapping (Inside)

Inside does not fetch from a central API. Initial seeds arrive via:

1. Signal from a trusted Outside helper (`snb://v2:` bundle)
2. QR code (in person)
3. Bluetooth mesh (from nearby Inside peers during blackout)

## Android notes

The release pipeline can publish a signed `app-release.apk`. For high-risk environments, verify `checksums.txt` and `checksums.sig` before sideloading.
