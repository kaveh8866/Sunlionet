# Dashboard Runtime Integration

## Overview

The web dashboard reads live runtime state from the local Inside agent over a localhost-only HTTP API.

- Inside agent binds the runtime API to `127.0.0.1` only
- The Next.js app proxies requests server-side to avoid browser localhost and CORS issues
- The Runtime dashboard polls every 3 seconds

## Runtime API (Inside)

Bind locally (localhost only):

- `GET /api/health`
- `GET /api/state`
- `GET /api/events`

State shape:

```json
{
  "status": "connected",
  "activeProfile": "reality-1",
  "latencyMs": 120,
  "lastUpdated": 1710000000,
  "failures": [],
  "mode": "real"
}
```

Event shape:

```json
{
  "timestamp": 1710000000,
  "type": "PROFILE_SWITCH",
  "message": "Switched to reality-1"
}
```

## Starting the Runtime API

Run the Inside agent with the runtime API enabled:

```bash
go run ./cmd/inside --runtime-api-addr 127.0.0.1:8080 --runtime-api-keepalive ...
```

Notes:

- `--runtime-api-addr` is optional; when not set, the API server is disabled
- `--runtime-api-keepalive` keeps the process alive after successful startup so the dashboard can connect

## Next.js Proxy Layer

The browser calls the Next.js API routes:

- `GET /api/proxy/health` → `http://127.0.0.1:8080/api/health`
- `GET /api/proxy/state` → `http://127.0.0.1:8080/api/state`
- `GET /api/proxy/events` → `http://127.0.0.1:8080/api/events`

To override the upstream base URL (server-side only):

- `SUNLIONET_RUNTIME_API_BASE=http://127.0.0.1:8080`
- `SUNLIONET_RUNTIME_API_BASE=http://127.0.0.1:8080` (legacy)

## Troubleshooting

- Dashboard shows “No active SunLionet runtime detected”:
  - Ensure the Inside agent is running with `--runtime-api-addr 127.0.0.1:8080 --runtime-api-keepalive`
  - Ensure nothing else is using port `8080`
- Proxy returns `runtime_unavailable`:
  - Confirm the Inside agent is bound to `127.0.0.1` (not `0.0.0.0`)
  - Check local firewall rules (localhost requests should not be blocked)

## Security Notes

- The runtime API refuses non-local bind addresses (only `127.0.0.1` / `localhost`)
- Endpoints are read-only and return operational metadata only (no keys, no secrets)
