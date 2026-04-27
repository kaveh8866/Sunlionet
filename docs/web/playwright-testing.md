# Web Playwright Testing

This document defines end-to-end coverage for user-facing web flows.

## Test Location

- `website/e2e/landing.spec.ts`
- `website/e2e/dashboard.spec.ts`

## Covered Flows

### Public pages

- Homepage smoke: loads, nav visible, core CTA visible
- Language toggle: EN/FA switch is deterministic on public routes
- Docs landing: page loads, docs navigation visible, docs route (`/docs/all`) reachable
- Download page: recommendation block visible, platform selector visible, verify/install sections visible
- Download → Verification guide: `/docs/outside/verification` is reachable from the primary CTA (EN/FA)
- Download fallback: selecting `Source code` renders a safe no-matching-artifact empty state
- Support page key trust sections render
- Navigation consistency across core pages (`/download`, `/docs`, `/support`)

### Dashboard

- Runtime page without runtime API: fallback message is shown
- Runtime page with injected connected state: connected badge + timeline event visible
- Runtime page with injected failure state: error badge + failure event visible
- Global/protocol dashboard pages: no-live-feed fallback remains stable

## Run Playwright

From `website/`:

```bash
npm run test:e2e
```

Optional pre-check:

```bash
npm run lint
```

## Determinism Notes

- Tests do not depend on external internet endpoints.
- Runtime/dashboard tests use Playwright request interception for deterministic state.
- Stable `data-testid` hooks are used for nav/docs/download controls where needed.

## Known Limitations

- Browser visual regressions are not currently snapshot-tested.
- Mobile-layout behavior is validated functionally, not with full visual baseline diffs.
