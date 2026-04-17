# Linux Installation

This guide provides three installation paths for ShadowNet on Linux:

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
BASE_URL="https://github.com/kaveh8866/shadownet-agent/releases/latest/download"
curl -fsSL "${BASE_URL}/shadownet-linux-amd64.tar.gz" | tar -xz
sudo install -m 0755 shadownet-inside /usr/local/bin/shadownet-inside
sudo install -m 0755 shadownet-outside /usr/local/bin/shadownet-outside
shadownet-inside --help
```

Expected output (last command): usage/help text from `shadownet-inside`.

## 2) Verified Install (Recommended)

This path verifies both file integrity and signed checksum metadata.

```bash
VERSION="v0.1.0"
BASE_URL="https://github.com/kaveh8866/shadownet-agent/releases/download/${VERSION}"

wget "${BASE_URL}/shadownet-linux-amd64.tar.gz"
wget "${BASE_URL}/checksums.txt"
wget "${BASE_URL}/checksums.sig"
wget "${BASE_URL}/checksums.pub"

grep ' shadownet-linux-amd64.tar.gz$' checksums.txt > shadownet-linux-amd64.tar.gz.sha256
sha256sum -c shadownet-linux-amd64.tar.gz.sha256
cosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt

tar -xzf shadownet-linux-amd64.tar.gz
sudo install -m 0755 shadownet-inside /usr/local/bin/shadownet-inside
sudo install -m 0755 shadownet-outside /usr/local/bin/shadownet-outside
```

Expected verification output:

- `shadownet-linux-amd64.tar.gz: OK`
- `Verified OK` (or equivalent success message from cosign)

## 3) Debian Package Install

```bash
VERSION="0.1.0"
BASE_URL="https://github.com/kaveh8866/shadownet-agent/releases/download/v${VERSION}"
wget "${BASE_URL}/shadownet_${VERSION}_amd64.deb"
sudo dpkg -i "shadownet_${VERSION}_amd64.deb"
```

Uninstall:

```bash
sudo dpkg -r shadownet
```

## 4) Developer Build

```bash
git clone https://github.com/kaveh8866/shadownet-agent.git
cd shadownet-agent
go build -trimpath -tags inside -ldflags="-s -w -X main.version=v0.1.0" -o shadownet-inside ./cmd/inside
go build -trimpath -tags outside -ldflags="-s -w -X main.version=v0.1.0" -o shadownet-outside ./cmd/outside
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
