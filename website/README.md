# SunLionet Website + Dashboard

This directory contains the SunLionet Next.js site, including:

- Download pages served from `public/downloads/…` (static artifacts + checksums).
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

CI runs lint + build for this directory (static export build).
