# Real-World Deployment Model (Resilient)

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

### Controlled Rollout (Phase 2)

- Start with a trusted cohort of `5-20` testers only.
- Use trusted contacts and encrypted transfer channels.
- Keep distribution closed until repeated failures are classified and fixed.
- Do not publish mass-download links during controlled rollout.

Operational controls:

1. Ship a clearly marked tester build (`SunLionet (Test Build)`).
2. Collect only manual, user-controlled feedback exports (`logs.json`).
3. Aggregate only high-level failure categories (no sensitive endpoints).
4. Patch and redeploy in short cycles (`24-72` hours).

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
- During controlled rollout, prefer encrypted one-to-one delivery instead of public channels.

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
- Diagnostics are opt-in and disabled by default.
- Never collect IP address, domain names, full configs, bundle contents, or user identity.

## Deployment Checklist

- Build signed APK + Linux binaries.
- Generate and sign manifest + checksums.
- Verify signatures from clean environment.
- Confirm installer/import/update paths in offline mode.
- Ensure at least two mirrors and one peer channel ready.
- Confirm tester mode flags/version labels are visible in-app.
- Confirm `logs.json` export and local `last_errors.json` diagnostics are functional.
