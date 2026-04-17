#!/usr/bin/env sh
set -eu

VERSION="${1:-v0.1.0}"
BASE_URL="${BASE_URL:-https://github.com/kaveh8866/shadownet-agent/releases/download/${VERSION}}"
ARCHIVE="shadownet-linux-amd64.tar.gz"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
cd "$tmpdir"

command -v cosign >/dev/null 2>&1 || {
  echo "cosign is required for verified install. Install it first: https://docs.sigstore.dev/cosign/system_config/installation/" >&2
  exit 1
}

curl -fsSLO "${BASE_URL}/${ARCHIVE}"
curl -fsSLO "${BASE_URL}/checksums.txt"
curl -fsSLO "${BASE_URL}/checksums.sig"
curl -fsSLO "${BASE_URL}/checksums.pub"

grep " ${ARCHIVE}\$" checksums.txt > "${ARCHIVE}.sha256"
sha256sum -c "${ARCHIVE}.sha256"
cosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt

tar -xzf "${ARCHIVE}"
sudo install -m 0755 shadownet-inside /usr/local/bin/shadownet-inside
sudo install -m 0755 shadownet-outside /usr/local/bin/shadownet-outside
echo "verified and installed shadownet binaries to /usr/local/bin"
