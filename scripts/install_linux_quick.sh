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

uname_m="$(uname -m | tr '[:upper:]' '[:lower:]')"
arch="amd64"
if [ "$uname_m" = "aarch64" ] || [ "$uname_m" = "arm64" ]; then
  arch="arm64"
fi

ARCHIVE="sunlionet-${ROLE}-${VERSION}-linux-${arch}.tar.gz"

if [ "$ROLE" != "inside" ] && [ "$ROLE" != "outside" ]; then
  echo "usage: $0 [inside|outside]" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

curl -fsSL "${BASE_URL}/${ARCHIVE}" | tar -xz -C "$tmpdir"
if [ -f "${tmpdir}/sunlionet-inside" ]; then
  sudo install -m 0755 "${tmpdir}/sunlionet-inside" /usr/local/bin/sunlionet-inside
elif [ -f "${tmpdir}/SUNLIONET-inside" ]; then
  sudo install -m 0755 "${tmpdir}/SUNLIONET-inside" /usr/local/bin/sunlionet-inside
fi

if [ -f "${tmpdir}/sunlionet-outside" ]; then
  sudo install -m 0755 "${tmpdir}/sunlionet-outside" /usr/local/bin/sunlionet-outside
elif [ -f "${tmpdir}/SUNLIONET-outside" ]; then
  sudo install -m 0755 "${tmpdir}/SUNLIONET-outside" /usr/local/bin/sunlionet-outside
fi

if command -v ln >/dev/null 2>&1; then
  if [ -f /usr/local/bin/sunlionet-inside ]; then sudo ln -sf sunlionet-inside /usr/local/bin/SUNLIONET-inside 2>/dev/null || true; fi
  if [ -f /usr/local/bin/sunlionet-outside ]; then sudo ln -sf sunlionet-outside /usr/local/bin/SUNLIONET-outside 2>/dev/null || true; fi
fi
echo "installed binaries to /usr/local/bin"
echo "run: sunlionet-${ROLE}"
