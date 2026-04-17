# Packaging Test Checklist

## Linux

- [ ] Download `shadownet-linux-amd64.tar.gz` and verify with `checksums.txt` + `checksums.sig`.
- [ ] Extract tarball and run:
  - [ ] `./shadownet-inside --help`
  - [ ] `./shadownet-outside --help`
- [ ] Install Debian package:
  - [ ] `sudo dpkg -i shadownet_0.1.0_amd64.deb`
  - [ ] `shadownet-inside --help`
  - [ ] `shadownet-outside --help`
- [ ] Uninstall package:
  - [ ] `sudo dpkg -r shadownet`

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
