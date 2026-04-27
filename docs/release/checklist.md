# Release Candidate 1 (RC1) Checklist

This checklist is for hardening and validating SunLionet for its first Release Candidate (RC1).

## RC1 Specific Hardening (Prompt 7)

### 1. Robustness & Recovery
- [x] **Secure Storage Resilience**: `SecureStore` handles KeyStore corruption or missing master keys by offering a "Reset" flow in the UI instead of crashing.
- [x] **Connection Backoff**: `AgentService` implements exponential backoff and connection probe intervals to prevent thrashing on restricted networks.
- [x] **Recovery Escalation**: `AgentService` implements 3 levels of recovery (runtime restart, bridge restart, full service restart) before giving up.
- [x] **State Repository Isolation**: UI state is decoupled from secure secrets to ensure the UI remains responsive even if secure storage is initializing or failing.

### 2. User Experience & Trust
- [x] **Bilingual Support (EN/FA)**: Critical UI strings and notifications are localized in both English and Farsi.
- [x] **Wording Clarity**: Labels and status messages are consistent across Android and CLI.
- [x] **Transparency**: Error messages provide clear context (e.g., "Network may be restricted") rather than just technical codes.

### 3. Packaging & Distribution
- [x] **Release Manifest**: `RELEASES.json` is generated alongside artifacts for website integration.
- [x] **Checksum Integrity**: Every artifact has a corresponding `.sha256` file.
- [x] **Version Consistency**: Version strings are injected into Go binaries and reflected in `VERSION.txt`.

## Standard Release Blockers (Must be True)

### Secrets and Sensitive Data
- [ ] No plaintext secrets at rest where prohibited.
- [ ] No secrets/tokens/seed material in logs.
- [ ] Repo passes secret scanning (`gitleaks`).

### Bundle Import and Trust
- [ ] Import validation is explicit and testable (Signature verification).
- [ ] Signer trust model is documented.

### Android Security Boundaries
- [ ] VPN consent is explicit user action.
- [ ] Stored secrets remain in encrypted storage.
- [ ] APK depends on checked-in gomobile artifacts under `android/app/libs/`.

## Artifacts and Verification

- [ ] Linux artifacts built (Inside/Outside).
- [ ] Android APK built and signed.
- [ ] `RELEASES.json` and `VERSION.txt` present.
- [ ] Linux install path tested (`install-linux.sh`).
- [ ] Android install + VPN flow tested.
- [ ] Bundle import + connection test works.

## Documentation

- [ ] README.md points to `docs/getting-started.md`.
- [ ] Security model describes trust boundaries.
- [ ] Troubleshooting covers common failure modes.

## Go/No-Go Criteria

1. **Security**: No unencrypted secrets on disk. All bundles must be signed and verified. (Must Pass)
2. **Core Journey**: User can import a bundle, connect, and see a "Connected" status on a real device. (Must Pass)
3. **Stability**: No app crashes during 1 hour of background connection with intermittent network. (Must Pass)
4. **Packaging**: All artifacts listed in `RELEASES.json` are downloadable and checksums match. (Must Pass)

