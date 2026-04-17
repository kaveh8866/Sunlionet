# Android Installation

ShadowNet Android release is distributed as a signed APK (`app-release.apk`).

> Version in examples: `v0.1.0`

## Prerequisites

- Android device (Android 8+ recommended)
- Internet access to fetch release artifacts
- Optional: a second trusted channel to confirm key fingerprint

## 1) Download APK

```bash
VERSION="v0.1.0"
BASE_URL="https://github.com/kaveh8866/shadownet-agent/releases/download/${VERSION}"
wget "${BASE_URL}/app-release.apk"
```

## 2) Optional Verification (Recommended)

```bash
wget "${BASE_URL}/checksums.txt"
wget "${BASE_URL}/checksums.sig"
wget "${BASE_URL}/checksums.pub"

grep ' app-release.apk$' checksums.txt > app-release.apk.sha256
sha256sum -c app-release.apk.sha256
cosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt
```

Expected output:

- `app-release.apk: OK`
- successful cosign verification message

## 3) Install APK (Sideload)

1. Transfer `app-release.apk` to your Android phone.
2. Open the file and start install.
3. If blocked, enable "Install unknown apps" only for the installer you use (Files browser or browser app).
4. Complete installation, then disable unknown-app install permission again.

## 4) First Launch and Setup

1. Open ShadowNet.
2. Grant VPN permission when prompted.
3. Import trusted bundle(s) and start the agent.

## Safety Warnings

- Install APK only from trusted release sources.
- Verify checksum/signature before sideloading in high-risk environments.
- Unknown-source installation increases risk; keep it enabled only during install.
- Prefer confirming release key fingerprint via an out-of-band channel.

## Troubleshooting

- APK install blocked:
  - confirm unknown-app install is enabled for the correct app
  - ensure APK is fully downloaded
- Verification failure:
  - re-download APK and metadata files
  - confirm files are from the same release tag
- App starts but VPN cannot connect:
  - verify imported profile validity
  - check local network restrictions and Android battery optimization settings
