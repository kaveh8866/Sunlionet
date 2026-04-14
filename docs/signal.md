# Signal Integration Protocol (Outside → Inside)

Signal is the primary distribution channel because it provides end-to-end encryption and a familiar UX for users.

## Roles

- Outside: sender of seed bundles (default)
- Inside: receiver-only by default

## Outside sending flow

1. Generate or ingest new seed profiles.
2. Validate profiles (basic reachability + template sanity).
3. Package profiles into a bundle, encrypt with age (X25519), and sign with Ed25519.
4. Send as a Signal message containing a single URI: `snb://v2:<base64url(wrapper_json)>`.
5. Use disappearing messages when possible.

## Inside receiving flow

1. Listen only for messages from explicitly trusted senders.
2. Extract the `snb://` URI from message text.
3. Verify signature against the trusted key list.
4. Check timestamps/expiry.
5. Decrypt payload (v2).
6. Merge profiles into the encrypted local store.
7. Apply revocations immediately.

## Optional Inside health reports

Default is receive-only. If the user explicitly enables it, Inside may send a minimal health report:

- aggregate success rate by protocol family
- coarse failure reasons (e.g., UDP blocked, SNI reset suspected)
- no domains, no visited traffic metadata, no stable identifiers
