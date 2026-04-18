# ShadowNet Tester Guide (Controlled Rollout)

## Scope

- This guide is for a trusted tester cohort only (`5-20` users).
- Use only controlled distribution channels:
  - direct APK sharing
  - encrypted bundle transfer
  - trusted contacts
- Do not repost publicly and do not forward unknown download links.

## Safety Notice

- This is a test build and may be unstable.
- Do not rely on it for critical communication.
- Do not share personal identity details in issue reports.

## Install Safely

1. Receive APK from a trusted coordinator.
2. Verify checksum/signature if provided.
3. Install manually from local file.
4. Open app and confirm build label: `ShadowNet (Test Build)`.
5. Confirm version shown in app (example: `v0.1.0-test3`).

## Import Bundle

1. Tap `Import Bundle`.
2. Select the encrypted bundle file provided by your coordinator.
3. Wait for import confirmation in app status.

## Connect

1. Tap `Connect`.
2. Accept VPN permission prompt.
3. Wait for status to become connected.
4. If connection fails, check `Last error` in app.

## If It Fails

1. Keep app open for a short period to let retries complete.
2. Record only high-level context:
   - Wi-Fi or mobile data
   - approximate time
   - failure category (DNS_BLOCKED/TCP_RESET/TLS_BLOCKED/TIMEOUT/NO_ROUTE/UNKNOWN)
3. Do not include:
   - IP addresses
   - domains
   - personal identifiers
   - raw configuration files

Failure categories used by test builds:

- `DNS_BLOCKED`
- `TCP_RESET`
- `TLS_BLOCKED`
- `TIMEOUT`
- `NO_ROUTE`
- `UNKNOWN`

## Report Issues (Manual Only)

1. Tap `Export Logs` to generate `logs.json`.
2. Review file before sharing.
3. Share manually with trusted coordinator.
4. Optional: tap `Report Issue` to open an email/message draft with `logs.json`.

Feedback loop:

1. issue occurs
2. tap `Export Logs`
3. send file manually
4. coordinator classifies pattern
5. next tester build is delivered

## Privacy Rules

- `Share anonymous diagnostics` is opt-in and defaults to OFF.
- Diagnostics include only minimal events (for example: connection failure category and retry count).
- No automatic upload is performed by the app.
- Errors are stored locally in `last_errors.json` for user-visible troubleshooting.
- Android runtime events may include `APP_KILLED`, `VPN_RESTART`, `VPN_DISCONNECT`, `NETWORK_SWITCH`, and `BATTERY_RESTRICTED`.

## Rapid Feedback Loop

- Iteration target: every `24-72` hours.
- Loop:
  1. collect tester reports
  2. classify recurring failures
  3. patch and validate
  4. release next test build
  5. repeat
