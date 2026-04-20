## Security Policy

### Supported Versions

Security fixes are applied to the `main` branch and the latest release tag.

| Version | Supported |
| ------- | --------- |
| Latest release | :white_check_mark: |
| Older releases | :x: |

### Reporting a Vulnerability

Do not open public issues for security reports.

Report vulnerabilities privately via:
- GitHub Security Advisories: use the repository "Report a vulnerability" flow.
- Email: `security@sunlionet.org` (include reproduction steps and impact).

Expected response targets:
- Initial triage acknowledgement: within 72 hours.
- Mitigation plan: within 7 days for confirmed issues.
- Coordinated disclosure after a fix is available.

### Secret Handling Requirements

- Never commit credentials, API tokens, key files, or `.vercel*` auth files.
- Rotate any leaked credential immediately.
- Revoke compromised tokens before opening a PR that references the incident.

### Hardening Baseline

- CI must pass `go test`, `go vet`, formatting checks, `govulncheck`, and secret scanning before merge.
- Dependencies are maintained through automated update PRs and security alerts.
