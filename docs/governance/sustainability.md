# SunLionet Sustainability and Maintenance Plan

## 1. Core Objectives
To ensure the long-term viability, security, and accessibility of SunLionet as an open-source tool, beyond the initial launch phase.

## 2. Maintenance Tiers and Priorities

### Tier 1: Critical Maintenance (P0/P1)
- **Scope**: Security vulnerabilities, core journey breakage, mirror outages.
- **Priority**: Highest. Interrupts all other work.
- **Cadence**: As needed (Hotfixes within 24-48 hours).
- **Ownership**: Security Response Team (SRT).

### Tier 2: Routine Maintenance
- **Scope**: Dependency updates, non-critical bug fixes, documentation corrections, CI/CD improvements.
- **Priority**: Medium. Scheduled weekly.
- **Cadence**: Bi-weekly releases (Beta/RC).
- **Ownership**: General Maintainers.

### Tier 3: Enhancement Work
- **Scope**: New features, UX improvements, performance optimizations.
- **Priority**: Low.
- **Cadence**: Major milestone releases (every 2-3 months).
- **Ownership**: Feature Leads and Contributors.

## 3. Component Maintenance Cadence

| Component | Task | Frequency |
| :--- | :--- | :--- |
| **Core (Go)** | `govulncheck` & Security Review | Monthly |
| **Android** | Gradle Dependency Update & Lint | Bi-weekly |
| **Mirrors** | Automated Health Check | Daily (Automated) |
| **Docs** | "Getting Started" Verification | Monthly |
| **Tests** | Simulation/Regression Pass | Every Release |

## 4. Support Sustainability

To avoid maintainer burnout and ensure scaling:
1. **Self-Service First**: Invest in searchable, bilingual (EN/FA) documentation and FAQs.
2. **Issue Hygiene**: Use automated templates for bug reports. Close issues that lack reproducible steps after 14 days of inactivity.
3. **Community Support**: Encourage the use of the community Telegram group for user-to-user help. Maintainers only intervene for confirmed bugs.
4. **Tooling**: Use "Diagnostic Export" in the app to allow users to provide structured logs without revealing sensitive info.

## 5. Handling Stale Components
- Components with no activity for 6 months are marked as "Maintenance Only."
- Components with known security issues that cannot be fixed within 30 days are "Deprecated" and removed from the default distribution.
