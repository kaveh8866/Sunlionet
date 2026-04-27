# Changelog

## [v0.3.0-rc1] - 2026-04-27

### Added
- **Bilingual Support (EN/FA)**: Full Farsi localization for the Android app and core documentation.
- **Hardened Recovery**: 3-level recovery loop in `AgentService` (runtime restart, bridge restart, full service restart).
- **Secure Storage Resilience**: `SecureStore` now handles KeyStore corruption and offers a manual reset flow.
- **Release Manifest**: Automated generation of `RELEASES.json` for website and mirror synchronization.
- **Bilingual Documentation**: Initial Farsi documentation for installation and safety.

### Changed
- **CI/CD Modernization**: Updated GitHub Actions to use `go.mod` for versioning and added Android quality gates (lint/unit tests).
- **State Management**: Centralized state reset in `SecureStore` for improved test isolation and reliability.
- **Go Core**: Refined connection probe intervals and backoff strategies for hostile networks.

### Fixed
- Fixed compilation errors in `SecureStore.kt` related to missing `java.io.File` imports.
- Fixed test flakiness in `MainActivityTest.kt` by ensuring a clean state before every test.
- Fixed incomplete bridge recovery logic in `AgentService.kt`.

### Security
- Integrated `gitleaks` into CI for secret scanning.
- Enforced strict signature verification for all imported bundles.
- Decoupled UI state from secure secrets to prevent sensitive data leaks in logs or screenshots.
