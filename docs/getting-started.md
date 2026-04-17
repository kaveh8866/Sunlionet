# Getting Started

ShadowNet is a local-first, bundle-based system designed for censorship-resilient connectivity in high-risk DPI environments.

It ships as:

- ShadowNet Inside: the on-device agent that selects profiles and manages the local data plane.
- ShadowNet Outside: the supporter tool that generates signed (and optionally encrypted) bundles for controlled delivery.
- ShadowNet App: the Android wrapper for ShadowNet Inside (VPN integration + bundle import UI).
- ShadowNet Dashboard: a web UI that can show local runtime status (no cloud required).

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

curl -fL -O "${BASE_URL}/shadownet-inside-${VERSION}-linux-amd64.tar.gz"
curl -fL -O "${BASE_URL}/shadownet-inside-${VERSION}-linux-amd64.tar.gz.sha256"
sha256sum -c "shadownet-inside-${VERSION}-linux-amd64.tar.gz.sha256"

tar -xzf "shadownet-inside-${VERSION}-linux-amd64.tar.gz"
sudo ./install-linux.sh inside
```

Import a bundle and connect:

```bash
shadownet-inside \
  --import ./bundle.snb.json \
  --trusted-signer-pub-b64url "<PASTE_TRUSTED_SIGNER_PUB_B64URL>" \
  --age-identity "<PASTE_AGE_SECRET_KEY>" \
  --master-key "<32_BYTE_HEX_MASTER_KEY>" \
  --probe-url "https://example.com"
```

What to expect:

- `[ShadowNet] Starting agent...`
- `[Profile] Selected: ...`
- `[Connection] Testing...`
- `[Connection] SUCCESS` (when probe is enabled and succeeds)

## First-Time Setup (Android user path)

1. Install ShadowNet App (signed APK).
2. Open the app and grant VPN permission.
3. Import a trusted bundle (from a supporter or trusted channel).
4. Tap Connect.
5. Monitor status and last error in the app.

Start here:

- [Android Installation](install/android.md)
- [Bundle verification (Outside)](outside/verification.md)

## First-Time Setup (Web dashboard path)

ShadowNet Dashboard can show the local runtime state if ShadowNet Inside is running with the runtime API enabled.

1. Start ShadowNet Inside with runtime API enabled:

```bash
shadownet-inside --runtime-api-addr 127.0.0.1:8080 --runtime-api-keepalive ...
```

2. Open the dashboard runtime page:

- Website: `/dashboard/runtime`

If you see “No active ShadowNet runtime detected”, check that:

- ShadowNet Inside is running on the same machine as your browser.
- The runtime API address is `127.0.0.1:<port>` (localhost-only).

## Safety notes

- Prefer trusted, out-of-band verification of release keys and bundle publisher keys.
- Assume seized-device scenarios and minimize what you store locally.
- Never import bundles from unknown publishers.
