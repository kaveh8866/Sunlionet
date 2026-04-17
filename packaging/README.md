# Packaging

This directory contains packaging templates for release artifacts.

- `deb/DEBIAN/control`: Debian package metadata template.
- `deb/DEBIAN/postinst`, `deb/DEBIAN/postrm`: systemd reload hooks.
- `deb/etc/systemd/system/*.service`: optional service unit files.

Release CI (`.github/workflows/reusable-build-release.yml`) composes the final
package content from built binaries and these templates.
