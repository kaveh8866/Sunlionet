# SunLionet Incident Response and Rollback Plan

## 1. Objective
To provide a clear, deterministic procedure for responding to critical failures, security compromises, or distribution issues post-launch.

## 2. Severity Classification

| Level | Description | Example |
| :--- | :--- | :--- |
| **P0 (Critical)** | Security compromise or total service failure. | Private key leak, "connected" but no traffic, major privacy bug. |
| **P1 (High)** | Core journey broken for major segment. | App crash on specific Android version, broken bundle import. |
| **P2 (Medium)** | Degraded UX or performance. | High battery drain, UI glitch, slow connection times. |

## 3. Incident Playbook

### Scenario: Bad Release Artifact Discovered
- **Detection**: User reports of hash mismatch or install failure.
- **Response**:
    1. Immediately un-publish the release from GitHub and the website.
    2. Post an "Incident Alert" on the official Telegram channel.
    3. Investigate the CI/CD pipeline for corruption.
- **Rollback**: Re-point the `latest` tag in `RELEASES.json` to the previous stable version.

### Scenario: Critical Security Bug Found
- **Detection**: Internal audit or community bug bounty report.
- **Response**:
    1. Activate the security response team.
    2. Issue a "Security Advisory" with mitigation steps (e.g., "Disconnect immediately").
    3. Develop and verify a hotfix within 24 hours.
- **Hotfix**: Push a "point release" (e.g., `v0.3.1`) that only addresses the security flaw.

### Scenario: Compromised Mirror/Source
- **Detection**: Report of malicious APK or modified binary on a mirror.
- **Response**:
    1. Remove the mirror from the official distribution list.
    2. Broadcast the "Untrusted Mirror" warning on all official channels.
    3. Remind users to verify SHA-256 hashes against the official `RELEASES.json`.

## 4. Rollback Procedure (Android)
Since Android does not support automatic downgrades, a rollback requires:
1. **Communication**: Notify users to uninstall the current version and install the previous APK from the official archive.
2. **Compatibility**: Ensure the `SecureStore` state remains compatible or triggers a clean reset if downgrading.

## 5. Rollback Procedure (Linux)
1. Run the `install-linux.sh` script for the previous stable version.
2. The script overwrites the `/usr/local/bin/sunlionet-*` binaries and restarts the systemd services.

## 6. Communication Templates
- **Incident Start**: "We are investigating reports of [Issue]. Please [Precautionary Action] for now. Next update in [Time]."
- **Incident Resolved**: "Issue [ID] has been resolved in version [X.Y.Z]. Download at [Link]. We apologize for the disruption."

## 7. Decision Matrix (Go/No-Go for Hotfix)
- **Go**: Hotfix passes all CI tests + 15-minute manual smoke test on real device.
- **No-Go**: Hotfix introduces new P1 regressions or breaks existing secure state.
