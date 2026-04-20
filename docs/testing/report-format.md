# Telemetry-Free Report Format

## Goals

- Useful for debugging without leaking sensitive data.
- No identifiers, no IPs, no endpoints, no device identifiers.
- Coarsened timestamps for plausible deniability.

## File Name

- `report.json`
- `logs.json` (tester build manual export)

## Schema

- `schema`: `SUNLIONET.report.v1`

## Example

```json
{
  "schema": "SUNLIONET.report.v1",
  "app_version": "v0.1.0",
  "generated_at_unix": 1776280000,
  "generated_hour_unix": 1776280000,
  "go_version": "go1.23.0",
  "goos": "android",
  "goarch": "arm64",
  "mode": "inside",
  "errors": [],
  "summary": {
    "state_present": true,
    "profiles_loaded": 6,
    "selected_family": "reality",
    "sing_box_running": true,
    "selection_source": "policy",
    "selection_confidence": "0.85",
    "profile_families": {
      "reality": 3,
      "hysteria2": 2,
      "tuic": 1
    },
    "profile_health": {
      "cooldown_count": 1,
      "failing_count": 2
    }
  }
}
```

## Privacy Rules

- Never include:
  - IPs/domains/ports
  - profile IDs
  - file paths
  - persistent device identifiers
  - raw logs by default

## Sharing Guidance

- Share only if safe.
- Prefer sending `report.json` plus a short scenario description:
  - network type (Wi-Fi/mobile)
  - country/region only if safe
  - scenario (DPI/heavy blocking/offline)

## Tester Build Log Export (`logs.json`)

Used for controlled rollout triage. Generated only when the tester explicitly taps `Export Logs`.

### Minimal Schema (Example)

```json
{
  "schema": "SUNLIONET.logs.v1",
  "version": "v0.1.0-test3",
  "tester_mode": true,
  "share_anonymous_diagnostics": false,
  "generated_at": "2026-04-18T13:00:00Z",
  "events": [
    {
      "event": "connection_fail",
      "reason": "DNS_FAILURE",
      "retry_count": 2,
      "success": false,
      "ts": "2026-04-18T12:58:00Z"
    },
    {
      "event": "NETWORK_SWITCH",
      "ts": "2026-04-18T12:59:00Z"
    }
  ],
  "last_errors": [
    {
      "component": "agent",
      "reason": "DNS_BLOCKED",
      "ts": "2026-04-18T12:58:00Z"
    }
  ],
  "logs": [
    "12:58:00 [connection][WARN] failed reason=DNS_BLOCKED",
    "12:59:00 [network][INFO] switch detected"
  ]
}
```

### Hard Exclusions

- Never include IP addresses.
- Never include domain names or endpoint URLs.
- Never include full runtime configs or bundle contents.
- Never include personal identity fields.
