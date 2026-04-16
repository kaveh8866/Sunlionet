# Update Guide (Offline-Friendly, No Central Dependency)

## Supported Update Sources

- Primary release source (if reachable).
- Community mirrors.
- Local file transfer (Bluetooth, USB, SD card, messaging attachment).

## Secure Update Flow

1. Obtain signed release manifest and update package.
2. Verify manifest signature against trusted release key.
3. Verify package checksum listed in manifest.
4. Install update package.
5. Keep previous working package for rollback.

## Android Offline Update

1. Receive APK via local transfer.
2. Verify signature/checksum.
3. Install new APK manually.
4. Open app and confirm status.

## Linux Offline Update

1. Receive tarball + checksums/signature.
2. Verify artifacts.
3. Replace binary.
4. Restart service/process.

## Bundle Updates

- Import new signed bundle file.
- If bundle is encrypted, ensure correct age identity is configured.
- Invalid bundles are rejected by verifier.

## If Verification Fails

- Stop update immediately.
- Re-fetch from different trusted source.
- Compare key fingerprints with trusted community channel.

