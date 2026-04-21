# Security Audit Checklist

See also: [Security Boundaries](boundaries.md) and [Release Blockers](../release/checklist.md).

## Input Validation
- Verify all imported bundles with trusted signer keys only.
- Reject unsigned, tampered, expired, or schema-incompatible bundles.
- Reject duplicate and conflicting profiles during import.
- Validate profile protocol, endpoint host/port, and required credential fields.
- Reject malformed JSON templates and invalid rendered outbound JSON.

## Trust Model
- Maintain an allowlist of trusted signer key IDs.
- Enforce strict signer checks at import boundaries.
- Reject unknown signers in strict mode.
- Enforce replay detection by rejecting already-seen bundle IDs.
- Maintain optional revocation lists for signer IDs and bundle IDs.

## Safe Defaults
- Disable auto-connect on first launch until explicit user action.
- Require bundle verification before import/use.
- Keep direct-route bypass disabled by default.
- Require explicit failures instead of silent fallback behavior.

## Runtime Guards
- Validate actions before execution (known profile, allowed action).
- Enforce profile switch rate limits to prevent thrashing.
- Enforce loop protection for repeated profile retries.
- Stop execution with explicit actionable errors when guards fail.

## Logging Hygiene
- Never log private keys, secret tokens, full bundle contents, or full endpoints.
- Sanitize runtime events and metadata before storing/broadcasting.
- Disable verbose Android logs in release builds.
- Keep error messages bounded and redacted.

## Android Hardening
- Use Android Keystore-backed encrypted storage for secrets.
- Disallow plaintext secret storage fallback.
- Sanitize sing-box stdout/stderr before UI logging.
- Keep debug logging disabled in release builds.

## Network Safety
- Avoid direct fallback routes unless explicitly enabled.
- Route DNS through protected outbound by default.
- Prevent unintended direct traffic leakage under normal operation.

## Test Coverage
- Invalid bundle payload rejection.
- Tampered signature rejection.
- Invalid profile rejection.
- Render injection/unknown outbound type rejection.
- Replay bundle detection.

## Known Risks / Follow-ups
- Persist replay protection state across restarts.
- Add explicit revocation-list distribution and rotation workflow.
- Add end-to-end leak tests with packet-level assertions.
