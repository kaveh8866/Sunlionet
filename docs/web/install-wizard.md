# Web Install Wizard

## Deploy

### Local development

```bash
cd website
npm install
npm run dev
```

Open:

- Outside wizard: http://localhost:3001/installation/wizard
- Inside wizard: http://localhost:3001/dashboard/installation/wizard

### E2E tests (Playwright)

```bash
cd website
npm run test:e2e
```

### Production build

```bash
cd website
npm ci
npm run build
npm run start
```

## Routes

- Outside wizard
  - `/installation/wizard/:step`
  - `/en/installation/wizard/:step`
  - `/fa/installation/wizard/:step`
- Inside wizard
  - `/dashboard/installation/wizard/:step`

Steps:

- `welcome`
- `platform`
- `download`
- `verify`
- `configure`
- `finish`

## QA Checklist (Cross-Device)

### Android (Chrome)

- Tap targets are easy to hit (Back/Next/Done, step tabs).
- Step tabs scroll horizontally and remain usable in portrait and landscape.
- Safe area padding prevents the bottom navigation from overlapping system UI.
- Reduced motion enabled in Android settings simplifies transitions without breaking layout.

### Windows (Chrome/Edge/Firefox)

- Keyboard navigation works: Tab reaches controls, Enter activates buttons, focus is visible after step transitions.
- Resize test: 360px width to desktop width without overflow or clipped content.
- High DPI and zoom: 125% and 200% zoom remain readable and usable.

### Linux (Chrome/Firefox)

- No layout shift on step transitions; progress bar animates smoothly.
- CPU-throttle test (DevTools): animations remain responsive, no stuck states.

### General

- Back/Next always navigates to the correct step.
- Direct deep-link loads work (open a step URL in a new tab).
- “Exit” returns to the correct parent area (Installation or Dashboard).
- Dark mode and light mode both keep contrast readable.

### Automated checks

- Playwright e2e suite passes locally: `npm run test:e2e`
