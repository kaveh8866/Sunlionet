# SunLionet Long-Term Risk Register

## 1. Technical Risks

| Risk | Impact | Mitigation Direction |
| :--- | :--- | :--- |
| **Protocol Detection** | High | Implement traffic obfuscation and rapid protocol rotation. |
| **State Corruption** | Medium | Periodic `SecureStore` backups and checksum verification. |
| **Dependency Vulnerability** | Medium | Monthly automated `govulncheck` and aggressive patching. |

## 2. Maintenance Risks

| Risk | Impact | Mitigation Direction |
| :--- | :--- | :--- |
| **Maintainer Burnout** | High | "Sustainability Model" (Tiered maintenance) and contributor onboarding. |
| **Stale Documentation** | Medium | Monthly verification of "Getting Started" guides against latest release. |
| **Infrastructure Costs** | Low | Decentralized hosting (IPFS/Arweave) and grant/donation seeking. |

## 3. Security Risks

| Risk | Impact | Mitigation Direction |
| :--- | :--- | :--- |
| **Release Key Compromise** | Critical | HSM/Offline storage, annual rotation, and 2-of-N signatures. |
| **Privacy Leak (Logs)** | High | Strict "No-Logs" policy and in-app diagnostic scrubbing. |
| **Malicious Contribution** | Medium | Mandatory multi-maintainer review for security-sensitive modules. |

## 4. Distribution Risks

| Risk | Impact | Mitigation Direction |
| :--- | :--- | :--- |
| **Global Domain Seizure** | High | Mirror tiering (Telegram, IPFS, Onion) and decentralized DNS. |
| **App Store Removal** | Medium | Focus on direct APK distribution and F-Droid as primary channels. |
| **Hostile Mirror Hijack** | High | Signed `RELEASES.json` and in-app signature verification of all artifacts. |

## 5. Governance Risks

| Risk | Impact | Mitigation Direction |
| :--- | :--- | :--- |
| **Centralization of Authority** | Medium | Lightweight governance model with explicit "Conflict Resolution" paths. |
| **Project Fork Fragmentation** | Low | Encouraging safe forks while maintaining a clear "Official" trust domain. |
| **Legal/Regulator Pressure** | High | Distributed maintainer set across different jurisdictions. |
