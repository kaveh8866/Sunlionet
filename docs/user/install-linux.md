# Install SunLionet on Linux (User Guide)

## Requirements

- Linux x86_64 or arm64
- Downloaded `inside` binary package + checksum/signature data

## Verify Before Running

1. Download binary and checksum files from trusted source.
2. Verify checksum:
   - `sha256sum -c <checksums-file>`
3. Verify signature with trusted publisher key (if provided in your release workflow).

## Install

1. Extract package:
   - `tar -xzf shadownet-inside-<version>-linux-<arch>.tar.gz`
2. Move binary:
   - `sudo install -m 0755 inside /usr/local/bin/shadownet-inside`
3. Prepare state dir:
   - `mkdir -p ~/.local/state/shadownet`

## First Run

1. Import bundle:
   - `shadownet-inside --master-key <32-byte-key> --import /path/to/bundle.snb.json --trusted-signer-pub-b64url <pub> --age-identity <identity> --render-only`
2. Validate config:
   - `shadownet-inside --master-key <32-byte-key> --validate-only`

## Optional systemd (advanced)

- Run as dedicated user.
- Keep logs local and rotate.
- Do not expose management ports publicly.
