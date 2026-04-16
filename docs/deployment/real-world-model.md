# Real-World Deployment Model (Censorship-Resilient)

## Threat Model

### Network-Level

- DPI and protocol fingerprinting against VPN-like traffic.
- UDP- or TLS-specific blocking waves.
- Domain/IP blocking of release endpoints and mirrors.

### Distribution-Level

- Primary download channels blocked or throttled.
- Mirrors removed or poisoned with tampered binaries.
- Messaging links disrupted.

### Device-Level

- Device inspection for app names/icons/log files.
- Forensic extraction after confiscation.

### User-Level

- Non-technical users skip verification.
- Panic operation errors (unsafe uninstall/log sharing behavior).

## Distribution Strategy

### Direct Channel

- GitHub Releases + static website release page.
- Signed manifest + checksums shipped with each release.

### Mirror Channel

- Community static mirrors (domain and direct IP variants).
- Mirror list bundled inside signed manifest.

### Peer Channel

- Bluetooth, local Wi-Fi transfer, USB/SD card.
- Offline package import path in app/CLI.

### Messaging Channel

- Session, SimpleX, Briar as transport for APK/update/bundle artifacts.
- Use detached signatures and chunked transfer for large files.

## Stealth Packaging

- **Neutral mode (default):** generic app label/icon, minimal wording.
- **Disguised mode (optional):** alternate launcher label/icon and hidden entry trigger.
- **Visible mode:** developer branding + verbose diagnostics.

## Update Mechanism (No Central Dependency)

1. Receive signed manifest from any source.
2. Verify manifest signature with pinned release key.
3. Select candidate source (direct/mirror/manual file).
4. Verify package checksum + signature.
5. Install/update.
6. Keep prior version for rollback.

Offline update is first-class via local file import.

## Safe Bundle Distribution

- Bundle signatures required.
- Optional recipient encryption (age).
- QR payload for small bundles.
- Chunking for message transport (`SNBCHUNK/1` format).

## Safety Model

- No anonymity guarantee.
- No legal guarantee.
- No telemetry or tracking.
- Local-only logs, bounded retention.
- Encrypted local state by default.

## Emergency Features

- Quick disconnect: immediate VPN and agent stop.
- Data wipe: clear local profiles/templates/runtime state.
- Hidden mode: optional stealth launcher behavior.

## Privacy Logging Rules

- Local-only.
- Rotated/bounded.
- No remote crash-reporting backend.

## Deployment Checklist

- Build signed APK + Linux binaries.
- Generate and sign manifest + checksums.
- Verify signatures from clean environment.
- Confirm installer/import/update paths in offline mode.
- Ensure at least two mirrors and one peer channel ready.

