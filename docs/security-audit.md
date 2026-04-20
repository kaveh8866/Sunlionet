# Security Audit Report

Date: 2026-04-20

## Scope

- Go codebase (agent, relay, storage)
- Website (Next.js)
- CI security controls (vulnerability + secret scanning)
- GitHub security signals (CodeQL, Dependabot, secret scanning)

## Executive Summary

- Remediated CodeQL high severity findings:
  - TLS certificate verification was disabled in DPI TLS probe code.
  - User-controlled values were used in filesystem path construction in the file-backed relay.
  - Error logging risked leaking sensitive information.
- Added/confirmed baseline security controls:
  - CI runs module vulnerability scanning (govulncheck) and secret scanning (gitleaks).
  - CodeQL scanning enabled for Go + JS/TS.
  - Dependabot configured for Go/npm/GitHub Actions updates.
- Remaining items require operational action:
  - Two GitHub secret scanning alerts for Vercel tokens require token revocation/rotation in Vercel.
  - GitHub Dependabot alert UI may lag after dependency upgrades; local govulncheck is clean.

## Findings and Fixes

### 1) TLS certificate verification disabled (High)

**Finding**

The TLS-based DPI/SNI probe used `InsecureSkipVerify: true`, which disables certificate verification.

**Fix**

Removed `InsecureSkipVerify` so the client uses normal certificate verification.

**Code**

- [dpi.go](file:///c:/Users/Kaveh/Desktop/Iran-Agent-Vpn/pkg/detector/dpi.go)

### 2) Uncontrolled data used in filesystem paths (High)

**Finding**

The file-backed relay used user-controlled identifiers in path construction. Even when joining paths, unvalidated IDs can enable path traversal or unexpected filesystem access.

**Fix**

- Mailbox directories now encode mailbox IDs using base64url to avoid path separator injection.
- Message IDs are validated as 16-byte base64url values before being used to generate filenames.

**Code**

- [file_relay.go](file:///c:/Users/Kaveh/Desktop/Iran-Agent-Vpn/pkg/relay/file_relay.go)
- [types.go](file:///c:/Users/Kaveh/Desktop/Iran-Agent-Vpn/pkg/relay/types.go)

### 3) Potential logging of sensitive information (High)

**Finding**

Certain error paths logged raw error strings which may contain sensitive material (e.g., config contents or embedded secrets).

**Fix**

Sanitized logging to avoid printing raw errors in sensitive paths.

**Code**

- [rotation.go](file:///c:/Users/Kaveh/Desktop/Iran-Agent-Vpn/pkg/policy/rotation.go)

### 4) Missing security headers / access control hardening (Medium)

**Finding**

Public-facing HTTP endpoints should provide baseline security headers and optional authentication to prevent unintended exposure if bound to non-local interfaces.

**Fix**

- Added baseline security headers for relay API JSON responses.
- Added optional `Authorization: Bearer <token>` authentication via `ServerOptions.AuthToken`.

**Code**

- [http_server.go](file:///c:/Users/Kaveh/Desktop/Iran-Agent-Vpn/pkg/relay/http_server.go)
- [next.config.ts](file:///c:/Users/Kaveh/Desktop/Iran-Agent-Vpn/website/next.config.ts)

## Web Security Review (Next.js)

- No SQL usage detected in the website code.
- No uses of `dangerouslySetInnerHTML` with user-controlled input; the single instance is a static theme initialization script:
  - [ThemeInitScript.tsx](file:///c:/Users/Kaveh/Desktop/Iran-Agent-Vpn/website/src/components/ThemeInitScript.tsx)
- Added baseline headers at the framework level (frame-ancestors, object-src, nosniff, deny framing).

## Dependency Security

- Go: `golang.org/x/crypto` upgraded to `v0.45.0` and `go` toolchain aligned to `1.24.0`.
- Website: dependency audit should be run on `website/` using the lockfile (`npm audit --omit=dev`).

## Secret Handling

- GitHub secret scanning shows Vercel tokens detected in `.vercel-global/auth.json`.
- This requires revoking/rotating the tokens in Vercel regardless of whether the file still exists in the repo history.

## Verification Performed

- Go: `go test ./...`, `go vet ./...`
- Go vuln scan: `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (expected: no vulnerabilities)
- CI controls: CodeQL + gitleaks + govulncheck configured

## Manual Pen Test Checklist (Recommended)

- Relay server:
  - Confirm `Authorization` is enforced when configured.
  - Fuzz JSON decoding (unknown fields, extra trailing JSON) and verify 400 responses.
  - Confirm large request bodies are rejected (MaxBytesReader).
- Website:
  - Confirm responses include the security headers.
  - Confirm no user-controlled HTML is rendered unsafely.
