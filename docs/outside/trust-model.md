# Outside: Trust Model

ShadowNet uses offline distribution of signed bundles rather than a central fetch API. This shifts trust to explicit key management and fail-closed verification.

## Actors

- Outside operator: curates seed profiles and produces bundles
- Inside device: imports bundles and applies them to the local secure store
- Transport: any channel capable of delivering text or a file (messengers, removable media, QR, local sharing)

## Trust Anchors

Inside must be configured with a set of trusted Ed25519 signer public keys. A bundle is accepted only if:

- its signature verifies under one of the trusted public keys
- its header and payload validations pass

No network lookup or “key server” is required.

## What the Signature Means

The Ed25519 signature authenticates:

- the entire bundle header (except the signature field itself)
- the ciphertext bytes (encrypted payload, or plaintext payload if `cipher="none"`)

This provides integrity and origin authentication. It does not provide confidentiality unless the payload is encrypted for a recipient.

## Recipient Encryption

If `cipher="age-x25519"`, the payload is encrypted for a specific age X25519 recipient public key.

Inside must possess the matching age identity (private key) to decrypt the payload. Verification checks that:

- `recipient_key_id` matches the identity’s recipient fingerprint
- decryption succeeds

## Fail-Closed Rules

Bundles are rejected if any trust-critical condition fails:

- invalid signature
- unknown cipher
- missing or inconsistent issuer metadata
- expired bundle
- malformed or non-canonical payload
- invalid profiles or missing required templates
- duplicates (IDs or endpoints) inside the payload

## Transport Is Not Trusted

The delivery channel is assumed to be:

- observable
- delayable
- modifiable
- sometimes unavailable

Therefore:

- integrity is provided by the signature
- confidentiality (if required) is provided by age encryption
- replay value is reduced by short bundle expirations

