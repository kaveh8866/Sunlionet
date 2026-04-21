# SunLionet Website + Dashboard

This directory contains the SunLionet Next.js site, including:

- Download pages served from `public/downloads/…` (static artifacts + checksums). The UI supports Linux, Windows (amd64), macOS (arm64), and Android (Termux CLI binary when no APK is published). iOS uses external links when `NEXT_PUBLIC_IOS_APPSTORE_URL` / `NEXT_PUBLIC_IOS_TESTFLIGHT_URL` are set. A machine-readable manifest is available at `/api/downloads`.
- Local dashboard UI that can read live runtime state from a localhost-only Inside runtime API via server-side proxy routes.
- Localized content (English + فارسی) sourced from `../content/` and `src/locales/`.

## Development

```bash
npm ci
npm run dev
```

Open http://localhost:3000.

## Runtime API Integration (Localhost Boundary)

The dashboard expects the Inside agent to expose a localhost-only runtime API, and proxies requests server-side to avoid browser localhost/CORS issues.

See: `../docs/dashboard/runtime-integration.md`.

## CI

CI runs lint + build for this directory (static export build). `npm run content:check` validates content parity and verifies that every artifact under `public/downloads/` has a matching `.sha256` file with the correct hash.
