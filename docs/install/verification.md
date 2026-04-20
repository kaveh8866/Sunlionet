# Artifact Verification

SunLionet release verification uses two layers:

1. File checksum validation (`sha256sum`)
2. Signed checksum verification (`cosign verify-blob`)

## Required Files

Download these files from the same release tag:

- target artifact (for example `SUNLIONET-linux-amd64.tar.gz` or `app-release.apk`)
- `checksums.txt`
- `checksums.sig`
- `checksums.pub`
- optional: `checksums.pub.sha256` (public key fingerprint file)

## Verification Commands

```bash
ARTIFACT="SUNLIONET-linux-amd64.tar.gz"
grep " ${ARTIFACT}\$" checksums.txt > "${ARTIFACT}.sha256"
sha256sum -c "${ARTIFACT}.sha256"
cosign verify-blob --key checksums.pub --signature checksums.sig checksums.txt
```

## Expected Output

- `ARTIFACT: OK` from `sha256sum -c`
- success confirmation from cosign signature verification

## Public Key Fingerprint

To check the release key fingerprint locally:

```bash
sha256sum checksums.pub
```

Compare output with published `checksums.pub.sha256` and a second trusted channel.

## Threat Model Notes

- Checksum match confirms integrity of the downloaded file.
- Signature validation confirms checksums were signed by release key owner.
- If attacker controls both binaries and the key distribution channel, verification can still be bypassed.
- Always use multiple channels for fingerprint confirmation in high-risk conditions.

## Troubleshooting

- `no such file`:
  - verify all metadata files were downloaded for the same release tag
- `sha256sum` mismatch:
  - re-download artifact and `checksums.txt`
- `cosign verify-blob` failure:
  - ensure `checksums.pub` matches published fingerprint
  - ensure `checksums.sig` was not truncated or modified
