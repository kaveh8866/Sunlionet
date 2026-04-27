# SunLionet Long-Term Release Strategy

## 1. Release Channels

| Channel | Stability | Target Audience | Update Cadence |
| :--- | :--- | :--- | :--- |
| **Stable** | High | General Users | 2-3 Months |
| **Beta / RC** | Medium | Testers / Early Adopters | 2-4 Weeks |
| **Edge / Nightly** | Low | Developers | Per Merge |

## 2. Update Philosophy
- **Security First**: Security patches bypass the standard release cadence and are delivered as hotfixes.
- **Minimal Disruption**: Avoid breaking changes to the bundle format or encrypted state (`SecureStore`) unless strictly necessary for safety.
- **Transparency**: Every release is accompanied by a signed `RELEASES.json` and a detailed Changelog in both English and Farsi.

## 3. Versioning (SemVer)
- **Major (X.0.0)**: Significant protocol changes, breaking state changes, or major UX shifts.
- **Minor (0.X.0)**: New features, performance improvements, or hardware support.
- **Patch (0.0.X)**: Bug fixes, security patches, or localization updates.

## 4. Security Patch Handling
- Security patches are released as "point releases" (e.g., `v1.0.1`).
- Critical (P0) patches trigger an "Emergency Update" notification in the app, urging users to update immediately.
- Patches for older major versions are provided for 6 months after the release of a new major version.

## 5. Compatibility and Deprecation
- **Backward Compatibility**: New versions of the app should support at least the previous 2 major versions of signed bundles.
- **Deprecation Policy**: Features or protocols marked for removal will be deprecated for one minor release before being removed.
- **Old Version Support**: Only the latest stable version and the previous stable version receive security updates.

## 6. Update Communication
- **GitHub**: Release notes and binary artifacts.
- **Telegram**: Summary of changes and direct APK/binary download links.
- **In-App**: "Update Available" notification with a link to the official mirror.
- **F-Droid**: Automatic update discovery for users using the F-Droid client.

## 7. Reproducible Builds (Goal)
- Our goal is to achieve 100% bit-for-bit reproducible builds for the Android APK and Linux binaries to enhance user trust and allow independent verification.
