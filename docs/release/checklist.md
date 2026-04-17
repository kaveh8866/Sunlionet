# Release Checklist

This checklist is for preparing and publishing a public ShadowNet release (example: `v0.1.0`).

## Artifacts

- ShadowNet Inside artifacts built for linux/amd64, linux/arm64, darwin/arm64, windows/amd64
- ShadowNet Outside artifacts built for linux/amd64, linux/arm64, darwin/arm64, windows/amd64
- Android APK built and signed (`app-release.apk`)
- Checksums generated for every artifact (`*.sha256` or `checksums.txt`)
- Checksum signatures generated and verified (`checksums.sig` + `checksums.pub`, if used)

## Verification

- Linux install path tested:
  - tarball extract + `install-linux.sh` works
  - systemd service installs and starts (if used)
- Bundle import tested (Inside):
  - valid bundle imports successfully
  - invalid signature fails closed with a clear error
- Connection test executed (Inside):
  - probe success path works
  - probe failure path produces a readable reason
- Dashboard tested:
  - runtime API starts on localhost only
  - `/dashboard/runtime` shows status + active profile + failures
- Android tested:
  - install + VPN permission flow works
  - import bundle flow works
  - connect/disconnect toggles state
  - last error is visible when failures occur

## Documentation

- README.md points to docs/getting-started.md as the single entry point
- Installation docs accurate for Linux and Android
- Bundle usage docs accurate (generate/verify/import)
- Troubleshooting covers common failure modes (missing keys, invalid signer key, sing-box missing)
- Security model describes trust boundaries without exaggerated claims

## Release process

- Version string set in build (`-X main.version=vX.Y.Z`)
- Tag created: `git tag vX.Y.Z`
- CI release workflow runs successfully for the tag
- GitHub release created with:
  - binaries + APK
  - checksums and signature metadata
  - short install instructions and verification steps
