#!/usr/bin/env sh
set -eu

ROLE="${1:-inside}"
BASE_URL="${BASE_URL:-https://github.com/kaveh8866/shadownet-agent/releases/latest/download}"
ARCHIVE="shadownet-linux-amd64.tar.gz"

if [ "$ROLE" != "inside" ] && [ "$ROLE" != "outside" ]; then
  echo "usage: $0 [inside|outside]" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

curl -fsSL "${BASE_URL}/${ARCHIVE}" | tar -xz -C "$tmpdir"
sudo install -m 0755 "${tmpdir}/shadownet-inside" /usr/local/bin/shadownet-inside
sudo install -m 0755 "${tmpdir}/shadownet-outside" /usr/local/bin/shadownet-outside
echo "installed binaries to /usr/local/bin"
echo "run: shadownet-${ROLE}"
