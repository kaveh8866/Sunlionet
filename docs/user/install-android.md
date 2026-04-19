# Install SunLionet on Android (User Guide)

## Safety First

- Install only from trusted channels you already verify.
- Compare file checksum before install.
- If uncertain, do not install.

## What You Need

- Android 8.0+ device (API 26+).
- APK file from trusted source (friend, mirror, USB, messaging app).
- Configuration bundle file from a trusted publisher.

## Step-by-Step

1. Receive APK file.
2. Verify checksum/signature using the published signed manifest.
3. Install APK (allow unknown sources only for this install action).
4. Open app and tap **Import configuration**.
5. Select your `.bundle` file from a trusted source.
6. Tap **Connect**.
7. Approve the VPN permission dialog.
8. Wait for status to show **Connected** (or a clear error).

## If It Fails

- `sing-box not found`: install build that includes proper sing-box asset.
- `bundle invalid`: verify you received correct signed bundle.
- `no profiles available`: import a valid bundle first.

## Panic / Risk Situations

- Use **Disconnect** immediately.
- Avoid sharing screenshots containing profile IDs or errors publicly.
- If device seizure risk is high, wipe app data from system settings.
