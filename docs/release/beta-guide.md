# ShadowNet Beta Guide (v0.2.0-beta)

This is a controlled beta release intended for sensitive environments. It may fail on some networks. Do not assume it is safe or reliable until you have personally verified it in your context.

## What “Beta” Means

- Expected: occasional connection failures, slower connects, missing features, and rough edges
- Not expected: silent behavior changes, silent updates, hidden telemetry, or “growth hacks”
- Data: ShadowNet should remain local-first; avoid depending on centralized telemetry

## Safety Basics

- Install only from a source you already trust (a person/community you can verify)
- Verify the file you received (checksum + signature) before installing
- Avoid sharing screenshots containing profile IDs, endpoints, or detailed error logs publicly

## Install (Android)

1. Receive the APK from a trusted channel.
2. Verify:
   - Checksum (SHA-256) matches the official release
   - Signature matches the release signing key for this beta
3. Install the APK (enable “unknown sources” only for this install if needed).
4. Open the app and confirm the label shows “ShadowNet Beta”.

## Quick Start

1. Import a configuration bundle (`.bundle`) using Import Bundle.
2. Tap Connect.
3. Approve the VPN permission prompt.
4. Wait for Connected status.

If it fails:
- Try again after switching networks (Wi‑Fi ↔ mobile data).
- Check your device battery restrictions; allow foreground notifications.

## Updates (No Silent Auto-Update)

ShadowNet Beta does not silently auto-update.

Update flow:
1. A “New version available” notice is shown (or shared by your distributor/community).
2. You choose whether to update.
3. You verify the update:
   - checksum matches the official release artifact
   - signature matches the official signing key
4. You install the update manually.

Rollback:
- Keep the previous known-good APK so you can revert if a new beta fails.

## Reporting Issues (Privacy-First)

Use GitHub Issues for non-sensitive reports. For sensitive regions or high-risk scenarios, use the private contact channel shared by your distributor/community.

When reporting, prefer “what happened” over personal details:
- Device model + Android version
- Network type (Wi‑Fi/mobile) and general region (country-level only if safe)
- Steps to reproduce
- Whether a bundle was imported and whether Connect was attempted

Suggested tags (include one):
- CONNECT_FAIL
- VPN_DROP
- APP_CRASH
- SLOW_CONNECT

Add the most relevant detail label:
- DNS_FAILURE
- TCP_RESET
- TLS_BLOCKED
- TIMEOUT
- NO_ROUTE
- CONFIG_ERROR
- BINARY_MISSING

Do not include:
- real endpoints, profile IDs, or bundle contents
- names, usernames, phone numbers, or precise location

## Known Limitations (Beta)

- Some networks may block or degrade connections.
- QR scanning may fail on low light or older devices.
- Connection speed can vary by profile and network conditions.

## When to Stop Using Beta

Stop rollout or revert to a previous build if you observe:
- repeated crashes on launch
- repeated VPN drops that impact safety
- unexpected prompts/behavior changes you cannot explain
- any indication of tampering with download artifacts

