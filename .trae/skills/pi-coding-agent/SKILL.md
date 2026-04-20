---
name: "pi-coding-agent"
description: "Installs the pi coding agent (from https://shittycodingagent.ai) and audits it plus this repo. Invoke when user asks to install that skill or run a security audit."
---

# Pi Coding Agent: Install + Security Audit

## Install

```bash
npm install -g @mariozechner/pi-coding-agent
```

## Verify

```bash
pi --version
```

## Audit Checklist

- Run repository scans: `go test ./...`, `go vet ./...`, `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- Run web audits (website/): `npm audit --omit=dev`, `npm run lint`, `npm run build`
- Review for injection and auth issues:
  - Go: validate all user-controlled inputs before using them in filesystem paths, commands, or templates
  - Web: avoid `dangerouslySetInnerHTML` with user-controlled data
- Ensure secrets are never committed; revoke any leaked credentials immediately
