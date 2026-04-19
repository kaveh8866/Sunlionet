# SunLionet: Website & Documentation Specification

This document details the structure, content, and design for the Next.js landing page and documentation site for the SunLionet project.

## 1. Folder Structure & Key Files

The website is built with Next.js 15 (App Router), Tailwind CSS, and standard React components.

```text
website/
├── public/
│   ├── arch-diagram.svg        # Interactive/static architecture diagram
│   ├── hero-bg.png             # Subtle dark-mode hero background
│   └── icon.png                # SunLionet Logo
├── src/
│   └── app/
│       ├── globals.css         # Tailwind directives and base styles
│       ├── layout.tsx          # Root layout (Header, Nav, Footer, Dark mode)
│       ├── page.tsx            # Home / Hero Section & Status Simulator
│       ├── architecture/
│       │   └── page.tsx        # Deep dive into Data/Control Plane
│       ├── docs/
│       │   ├── page.tsx        # Documentation index & setup wizard
│       │   ├── [slug]/page.tsx # Dynamic doc pages (e.g., /docs/security)
│       ├── download/
│       │   └── page.tsx        # OS-specific downloads & verification
│       └── roadmap/
│           └── page.tsx        # Future plans (mesh, synthetic data)
├── next.config.ts              # Contains `output: 'export'` for static hosting
├── tailwind.config.ts          # Custom colors (gold / dark blue)
└── package.json                # Dependencies (next, react, tailwindcss)
```

## 2. Page-by-Page Content Outline

### Home / Hero Section (`/`)
- **Tagline**: "Offline-first. Private. Resilient."
- **Copy**: "SunLionet is an offline-first, bundle-based system designed for privacy and resilient communication under network interference. It rotates local sing-box configurations on-device to reduce dependency on centralized services and manual intervention."
- **Visuals**: Animated "System Status Simulator" showing real-time fake logs of `detector.Event` -> `policy.Action` -> `sing-box.Reload`.
- **CTAs**: Primary "Download for Linux & Android", Secondary "Read the Docs".

### Architecture Page (`/architecture`)
- **Visual Diagram**: A responsive SVG or interactive flow chart showing the Outside daemon scraping and bundling configs, sending via Signal, and the Inside daemon running the Detector -> Policy Engine -> sing-box loop.
- **Copy**: Explanation of the "Control Plane" (Policy/LLM) vs "Data Plane" (sing-box). Details on how the dual-version split isolates risk.

### Documentation / Docs (`/docs`)
- **Setup Wizard**: Tabbed interface for Linux, Android, and Raspberry Pi.
- **Linux Setup**: link to verified install instructions hosted on the project site (no central fetch API for bundles).
- **Android Setup**: Instructions for sideloading the APK or running via Termux.
- **Security Model**: Detailed explanation of age-encryption, wipe-on-suspicion, and why the local DB keeps no domain logs.

### Download Page (`/download`)
- **Design**: Platform-specific cards (Linux, Android, Pi) with toggle for "Inside" vs "Outside" versions.
- **Content**: See "Download Section Specification" below.

### Future / Roadmap (`/roadmap`)
- **Phase 2**: Bluetooth Mesh for local peer proxy sharing during total internet blackouts.
- **Phase 3**: Synthetic data fine-tuning for the Phi-4-mini LLM Advisor to improve routing decisions without external API calls.

## 3. Download Section Specification

The download page must provide clear, secure, and verifiable artifacts.

### Linux (x86_64 / arm64)
- **Inside Binary**: `sunlionet-inside-linux-amd64-v1.0.0.tar.gz`
- **Outside Binary**: `sunlionet-outside-linux-amd64-v1.0.0.tar.gz`
- **Command**: 
  ```bash
  curl -LO <RELEASE_URL>/sunlionet-inside-linux-amd64-v1.0.0.tar.gz
  tar -xzf sunlionet-inside-linux-amd64-v1.0.0.tar.gz
  sudo ./install.sh
  ```
- **Verification**: Display `SHA256` hash and GPG signature instructions.
  ```bash
  sha256sum -c sunlionet-inside.sha256
  ```

### Android (APK)
- **File**: `sunlionet-inside-android-universal-v1.0.0.apk`
- **Command (Termux)**:
  ```bash
  pkg install wget
  wget <RELEASE_URL>/sunlionet-inside-termux-arm64.gz
  ```
- **Note**: For MVP distribution, users may need to sideload the APK and enable “Install unknown apps”.

### Raspberry Pi (ARM64)
- **File**: `sunlionet-inside-linux-arm64-v1.0.0.tar.gz`
- **Use Case**: Recommended for running as a permanent home router gateway for the family.

## 4. Adjustments to Dual-Version Design for the Website

To support a static, zero-telemetry website, the dual-version architecture requires a few operational adjustments:
1. **No Central API for Seeds**: The website cannot host live proxy seeds. If it did, the domain would be blocked instantly. The website is strictly for software distribution. Seed bootstrapping remains strictly peer-to-peer (Signal/Bluetooth).
2. **Static Export**: The Next.js app must use `output: "export"` in `next.config.ts`. This allows the site to be hosted on IPFS, GitHub Pages, Cloudflare Pages, or Tor, maximizing uptime against state-level DNS blocks.
3. **Wipe-on-Suspicion Integration**: The documentation must clearly explain that uninstalling the app is not enough; users must trigger the "Panic Button" (which invokes `store.WipeOnSuspicion()`) if physical device seizure is imminent.
