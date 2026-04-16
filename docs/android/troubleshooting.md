# Android Troubleshooting

## VPN Permission Denied

- Symptom: connect does nothing or returns to disconnected state.
- Fix: retry connect and approve VPN dialog.

## sing-box Not Found

- Symptom: logs show missing asset or startup error.
- Fix: add architecture-specific binary assets:
  - `assets/sing-box/arm64-v8a/sing-box`
  - `assets/sing-box/armeabi-v7a/sing-box`
- Ensure copied file is executable after install.

## No Profiles Available

- Symptom: status shows `no profiles available`.
- Fix: import a signed bundle via Import button.

## Bundle Invalid

- Symptom: import fails with verification/decryption errors.
- Fix: verify trusted signer key and age identity in Go bridge config.

## Frequent Restarts

- Symptom: sing-box repeatedly restarts.
- Fix:
  - validate rendered config,
  - inspect logs in app UI,
  - ensure profile endpoint is reachable.

## App Killed In Background

- Symptom: connection drops after system cleanup.
- Fix:
  - keep foreground notifications enabled,
  - disable aggressive battery optimizations for the app.

