# Packaging Test Checklist

## Linux

- [ ] Download `SUNLIONET-linux-amd64.tar.gz` and verify with `checksums.txt` + `checksums.sig`.
- [ ] Extract tarball and run:
  - [ ] `./SUNLIONET-inside --help`
  - [ ] `./SUNLIONET-outside --help`
- [ ] Install Debian package:
  - [ ] `sudo dpkg -i SUNLIONET_0.1.0_amd64.deb`
  - [ ] `SUNLIONET-inside --help`
  - [ ] `SUNLIONET-outside --help`
- [ ] Uninstall package:
  - [ ] `sudo dpkg -r SUNLIONET`

## Android

- [ ] Download `app-release.apk`.
- [ ] Verify APK hash against `checksums.txt`.
- [ ] Verify checksum signature with cosign.
- [ ] Sideload APK and launch app.
- [ ] Start VPN service from app UI.
- [ ] Import trusted bundle and confirm agent startup.

## Release Metadata

- [ ] `release.json` exists and includes all expected artifacts.
- [ ] `checksums.txt` includes tar/deb/apk entries.
- [ ] `checksums.sig` verifies with `checksums.pub`.
- [ ] `checksums.pub.sha256` matches locally computed fingerprint.
