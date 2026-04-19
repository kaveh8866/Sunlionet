# Getting Started

SunLionet is a local-first, bundle-based privacy and resilient communication system designed for high-risk, restricted networks.

It ships as:

- SunLionet Inside: the on-device agent that selects profiles and manages the local data plane.
- SunLionet Outside: the supporter tool that generates signed (and optionally encrypted) bundles for controlled delivery.
- SunLionet App: the Android wrapper for SunLionet Inside (VPN integration + configuration import UI).
- SunLionet Dashboard: a web UI that can show local runtime status (no cloud required).

Migration note: this repository is still named `shadownet-agent` during the transition, so some internal identifiers and older release artifacts may still use legacy `shadownet-*` naming.

## First-Time Setup (Linux user path)

1. Download a release artifact and its checksum.
2. Verify the checksum (and signature, if provided).
3. Install the binaries.
4. Import a trusted bundle.
5. Connect and confirm status.

Minimal flow (example for Linux amd64):

```bash
VERSION="v0.1.0"
BASE_URL="https://github.com/kaveh8866/shadownet-agent/releases/download/${VERSION}"

curl -fL -O "${BASE_URL}/sunlionet-inside-${VERSION}-linux-amd64.tar.gz"
curl -fL -O "${BASE_URL}/sunlionet-inside-${VERSION}-linux-amd64.tar.gz.sha256"
sha256sum -c "sunlionet-inside-${VERSION}-linux-amd64.tar.gz.sha256"

tar -xzf "sunlionet-inside-${VERSION}-linux-amd64.tar.gz"
sudo ./install-linux.sh inside
```

Import a bundle and connect:

```bash
sunlionet-inside \
  --import ./bundle.snb.json \
  --trusted-signer-pub-b64url "<PASTE_TRUSTED_SIGNER_PUB_B64URL>" \
  --age-identity "<PASTE_AGE_SECRET_KEY>" \
  --master-key "<32_BYTE_HEX_MASTER_KEY>" \
  --probe-url "https://example.com"
```

What to expect:

- `[SunLionet] Starting agent...`
- `[Profile] Selected: ...`
- `[Connection] Testing...`
- `[Connection] SUCCESS` (when probe is enabled and succeeds)

## First-Time Setup (Android user path)

1. Install SunLionet App (signed APK).
2. Open the app and grant VPN permission.
3. Import a trusted bundle (from a supporter or trusted channel).
4. Tap Connect.
5. Monitor status and last error in the app.

Start here:

- [Android Installation](install/android.md)
- [Bundle verification (Outside)](outside/verification.md)

## First-Time Setup (Web dashboard path)

SunLionet Dashboard can show the local runtime state if SunLionet Inside is running with the runtime API enabled.

1. Start SunLionet Inside with runtime API enabled:

```bash
sunlionet-inside --runtime-api-addr 127.0.0.1:8080 --runtime-api-keepalive ...
```

2. Open the dashboard runtime page:

- Website: `/dashboard/runtime`

If you see “No active SunLionet runtime detected”, check that:

- SunLionet Inside is running on the same machine as your browser.
- The runtime API address is `127.0.0.1:<port>` (localhost-only).

## Safety notes

- Prefer trusted, out-of-band verification of release keys and bundle publisher keys.
- Assume seized-device scenarios and minimize what you store locally.
- Never import bundles from unknown publishers.
