# Pi Orchestrator Integration (Local, Optional)

## Overview

Inside stays deterministic-first:

- Detector + Policy remain primary.
- Pi is consulted only when selection is ambiguous or when explicitly enabled.
- Pi never blocks core operation. Any Pi error, timeout, or invalid output falls back to Policy.
- Go enforces hard safety guards regardless of Pi output.

## JSON-RPC Contract

Transport:

- Line-delimited JSON over stdin/stdout (preferred), or
- Line-delimited JSON over TCP (localhost).

Method:

- `decide`

Request (Go → Pi):

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "decide",
  "params": {
    "type": "decision_request",
    "timestamp": 1710000000,
    "network": {
      "udp_blocked": true,
      "dns_poisoning": false,
      "latency_ms": 180
    },
    "profiles": [
      {
        "id": "p1",
        "protocol": "hysteria2",
        "last_success": 1709999000,
        "failures": 3,
        "cooldown": false
      },
      {
        "id": "p2",
        "protocol": "reality",
        "last_success": 1709998000,
        "failures": 0,
        "cooldown": false
      }
    ],
    "history": {
      "recent_switches": ["p1", "p1", "p1"],
      "fail_rate": 0.6
    },
    "constraints": {
      "max_switch_rate": 3,
      "no_udp": true
    }
  }
}
```

Response (Pi → Go):

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "action": "switch_profile",
    "profile_id": "p2",
    "reason": "UDP likely blocked, prefer TCP-based Reality",
    "confidence": 0.78,
    "fallback": ["p1"]
  }
}
```

Allowed actions:

- `switch_profile`
- `hold_current`
- `cooldown_profile`
- `import_bundle` (future-ready; currently ignored by Inside)
- `activate_mesh` (future-ready; currently ignored by Inside)

Any unknown fields, actions, or invalid values are rejected and trigger fallback.

## Inside CLI Flags

- `--use-pi`
- `--pi-endpoint host:port` (optional; if set, uses TCP; otherwise runs `--pi-cmd`)
- `--pi-cmd pi`
- `--pi-timeout-ms 1200`

## Pi Agent Prompt (Local Skill)

Suggested file to create on the machine running Pi:

`~/.pi/agent/sunlionet.md`

Content:

```markdown
You are the local SunLionet Pi Orchestrator. You only output a single JSON-RPC 2.0 response per request line.

Decision rules:
- If constraints.no_udp is true, do not choose UDP transports (hysteria2/tuic).
- Prefer profiles with recent last_success and fewer failures.
- Avoid loops: if history.recent_switches repeats the same profile, recommend a different viable profile if possible.
- If multiple profiles are viable, include resp.fallback as alternates.
- Never invent profile ids; only choose from params.profiles[].id.

Protocols:
- reality: TCP-based, generally safer when UDP is blocked.
- hysteria2: UDP-based, strong in some conditions but fragile under UDP blocking.
- tuic: UDP-based, similar risk under UDP blocking.

Output schema (result):
{
  "action": "switch_profile" | "hold_current" | "cooldown_profile" | "import_bundle" | "activate_mesh",
  "profile_id": "..." (required for switch_profile/cooldown_profile),
  "reason": "...",
  "confidence": 0.0-1.0,
  "fallback": ["..."]
}
```

## Safety Guarantees

Inside enforces:

- action must be one of the allowed set
- profile_id must exist in the provided candidates
- confidence must be within [0, 1]
- constraints.no_udp blocks UDP transport choices
