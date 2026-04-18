#!/usr/bin/env sh
set -eu

ROLE="${1:-inside}"
VERSION="${2:-v0.1.0}"
BASE_URL="${BASE_URL:-https://github.com/kaveh8866/sunlionet-core/releases/download/${VERSION}}"

uname_s="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "$uname_s" != "linux" ]; then
  echo "this installer is for Linux only" >&2
  exit 1
fi

uname_m="$(uname -m | tr '[:upper:]' '[:lower:]' )"
arch="amd64"
if [ "$uname_m" = "aarch64" ] || [ "$uname_m" = "arm64" ]; then
  arch="arm64"
fi

ARCHIVE="sunlionet-${ROLE}-${VERSION}-linux-${arch}.tar.gz"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
cd "$tmpdir"

curl -fsSLO "${BASE_URL}/${ARCHIVE}"
curl -fsSLO "${BASE_URL}/${ARCHIVE}.sha256"

sha256sum -c "${ARCHIVE}.sha256"

tar -xzf "${ARCHIVE}"
if [ -f "./sunlionet-inside" ]; then
  sudo install -m 0755 "./sunlionet-inside" /usr/local/bin/sunlionet-inside
elif [ -f "./shadownet-inside" ]; then
  sudo install -m 0755 "./shadownet-inside" /usr/local/bin/sunlionet-inside
fi

if [ -f "./sunlionet-outside" ]; then
  sudo install -m 0755 "./sunlionet-outside" /usr/local/bin/sunlionet-outside
elif [ -f "./shadownet-outside" ]; then
  sudo install -m 0755 "./shadownet-outside" /usr/local/bin/sunlionet-outside
fi

if command -v ln >/dev/null 2>&1; then
  if [ -f /usr/local/bin/sunlionet-inside ]; then sudo ln -sf sunlionet-inside /usr/local/bin/shadownet-inside 2>/dev/null || true; fi
  if [ -f /usr/local/bin/sunlionet-outside ]; then sudo ln -sf sunlionet-outside /usr/local/bin/shadownet-outside 2>/dev/null || true; fi
fi

echo "verified and installed sunlionet binaries to /usr/local/bin"
