# Installation Guide

This repository currently targets:

- ShadowNet-Inside: Linux laptops and Android (Termux for development)
- ShadowNet-Outside: any OS with stable internet (Linux/macOS/Windows)

## Build tags

- Inside: `-tags inside`
- Outside: `-tags outside`

## Development prerequisites

- Go 1.21+

Optional tools:

- `sing-box` (runtime for real connectivity)
- a local LLM runtime (e.g., llama.cpp) for the bounded advisor path

## Build

```bash
mkdir -p bin

go build -tags inside -ldflags="-s -w" -o bin/shadownet-inside ./cmd/inside/
go build -tags outside -ldflags="-s -w" -o bin/shadownet-outside ./cmd/outside/
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

1. Signal from a trusted Outside helper (`snb://v1:` bundle)
2. QR code (in person)
3. Bluetooth mesh (from nearby Inside peers during blackout)

