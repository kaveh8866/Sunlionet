# Telemetry-Free Report Format

## Goals

- Useful for debugging without leaking sensitive data.
- No identifiers, no IPs, no endpoints, no device identifiers.
- Coarsened timestamps for plausible deniability.

## File Name

- `report.json`

## Schema

- `schema`: `shadownet.report.v1`

## Example

```json
{
  "schema": "shadownet.report.v1",
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

