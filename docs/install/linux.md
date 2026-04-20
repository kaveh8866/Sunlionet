# Linux Installation

This guide provides three installation paths for SunLionet on Linux:

1. Quick install (fast path)
2. Verified install (recommended for hostile environments)
3. Developer build (from source)

> Version in examples: `v0.1.0`

## Prerequisites

- Linux amd64 host
- `curl`, `tar`, `sha256sum`
- `cosign` (for verified flow)
- `sudo` access for system install

## 1) Quick Install

This path is fast and suitable when you already trust your download channel.

```bash
BASE_URL="https://github.com/kaveh8866/SUNLIONET-agent/releases/latest/download"
curl -fsSL "${BASE_URL}/sunlionet-linux-amd64.tar.gz" | tar -xz
sudo install -m 0755 sunlionet-inside /usr/local/bin/sunlionet-inside
sudo install -m 0755 sunlionet-outside /usr/local/bin/sunlionet-outside
sunlionet-inside --help
```

If you are installing from older releases that still ship legacy artifact names, replace `sunlionet-...` with `SUNLIONET-...`.

Expected output (last command): usage/help text from `sunlionet-inside`.

## 2) Verified Install (Recommended)

This path verifies both file integrity and signed checksum metadata.

```bash
VERSION="v0.1.0"
BASE_URL="https://github.com/kaveh8866/SUNLIONET-agent/releases/download/${VERSION}"

wget "${BASE_URL}/sunlionet-linux-amd64.tar.gz"
wget "${BASE_URL}/checksums.txt"
wget "${BASE_URL}/checksums.sig"
wget "${BASE_URL}/checksums.pub"

grep ' sunlionet-linux-amd64.tar.gz$' checksums.txt > sunlionet-linux-amd64.tar.gz.sha256
sha256sum -c sunlionet-linux-amd64.tar.gz.sha256
cosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt

tar -xzf sunlionet-linux-amd64.tar.gz
sudo install -m 0755 sunlionet-inside /usr/local/bin/sunlionet-inside
sudo install -m 0755 sunlionet-outside /usr/local/bin/sunlionet-outside
```

Expected verification output:

- `sunlionet-linux-amd64.tar.gz: OK`
- `Verified OK` (or equivalent success message from cosign)

## 3) Debian Package Install

```bash
VERSION="0.1.0"
BASE_URL="https://github.com/kaveh8866/SUNLIONET-agent/releases/download/v${VERSION}"
wget "${BASE_URL}/sunlionet_${VERSION}_amd64.deb"
sudo dpkg -i "sunlionet_${VERSION}_amd64.deb"
```

Uninstall:

```bash
sudo dpkg -r sunlionet
```

## 4) Developer Build

```bash
git clone https://github.com/kaveh8866/SUNLIONET-agent.git
cd SUNLIONET-agent
go build -trimpath -tags inside -ldflags="-s -w -X main.version=v0.1.0" -o sunlionet-inside ./cmd/inside
go build -trimpath -tags outside -ldflags="-s -w -X main.version=v0.1.0" -o sunlionet-outside ./cmd/outside
```

## Troubleshooting

- `sha256sum` fails:
  - re-download the artifact and `checksums.txt`
  - confirm you matched the exact filename in `checksums.txt`
- `cosign verify-blob` fails:
  - confirm `checksums.pub` is the expected release key
  - verify key fingerprint via a second trusted channel
- `dpkg -i` dependency or permission errors:
  - run `sudo apt-get -f install` and retry
  - check systemd availability for service integration
