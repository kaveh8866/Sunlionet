# SunLionet Signed Onboarding URI

SunLionet supports a compact deep link for one-action bootstrapping:

```text
sunlionet://config/<base64url-envelope>
```

QR codes may use the shorter alphanumeric-friendly form `SL1:<base64url-envelope>`.
Multi-frame QR handoff uses `SLQR1:<index>/<total>:<fragment>` and clients join all
fragments before parsing. The joined payload must still fit the same envelope limits.

## Envelope

The payload is unpadded base64url over a binary envelope:

| Field | Size | Notes |
| --- | ---: | --- |
| Magic | 4 | ASCII `SLO` plus version byte `0x01` |
| IssuedAt | 4 | Big-endian Unix seconds |
| ExpiresAt | 4 | Big-endian Unix seconds, max 24 hour activation window |
| Family | 1 | `1=reality`, `2=hysteria2`, `3=tuic`, `4=shadowtls`, `5=dns_tunnel` |
| Port | 2 | Big-endian TCP/UDP port |
| SignerPub | 32 | Ed25519 public key that must already be trusted locally |
| Flags | 1 | Reserved, currently zero |
| Host | LV | 1-byte length then ASCII DNS name or IP, max 96 bytes |
| SNI | LV | Optional SNI override, max 96 bytes |
| CredentialA | LV | Family-specific compact credential |
| CredentialB | LV | Family-specific compact credential |
| CredentialC | LV | Family-specific compact credential |
| Tag | LV | Optional lowercase route tag |
| Signature | 64 | Ed25519 signature over exact unsigned bytes |

The signed message is:

```text
"SUNLIONET-ONBOARDING-V1\x00" || unsigned-envelope
```

The parser rejects links above 300 payload characters, expired links, not-yet-valid
links with more than five minutes of clock skew, untrusted signer keys, malformed
hosts, path separators, traversal-looking hostnames, and unsupported protocol
families. Nothing is written to local storage until the envelope signature and
all profile normalization checks pass.

## Credential Mapping

`reality`:

- `CredentialA`: raw 16-byte UUID.
- `CredentialB`: raw 32-byte Reality public key, rendered as base64url in the profile.
- `CredentialC`: raw 1-8 byte short ID, rendered as lowercase hex.

`hysteria2`:

- `CredentialA`: printable password, max 48 bytes.
- `CredentialB`: printable obfs password, max 48 bytes.

`tuic`:

- `CredentialA`: raw 16-byte UUID.
- `CredentialB`: printable password, max 48 bytes.

The Android wrapper only performs size and character sanitation. Cryptographic
verification and storage mutation are performed in Go via `ImportOnboardingURI`.
