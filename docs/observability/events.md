# Real-Time Observability (Events + SSE)

SunLionet Inside exposes a local-first, privacy-safe observability surface based on structured events and an SSE stream.

## Goals

- Real-time dashboard updates (no polling lag for events)
- Traceable decisions and failures (why a profile switched, why a connection failed)
- Local-first (localhost-only runtime API)
- Privacy-safe (no keys, no raw bundles, no secrets)

## Data Model

Every event is a JSON object:

```json
{
  "timestamp": 1710000000,
  "type": "PROFILE_SWITCH",
  "message": "Selected profile reality-1",
  "metadata": {
    "selected": "reality-1",
    "source": "policy",
    "confidence": 0.82,
    "reason": "score=..."
  }
}
```

- `timestamp`: Unix seconds
- `type`: stable identifier for filtering and coloring
- `message`: short human-readable summary
- `metadata`: small, structured details (must be safe)

## Event Types

Common event types emitted by the Inside agent:

- `AGENT_START`: agent started (includes mode)
- `API_LISTEN`: runtime API started (includes addr)
- `POLICY_DECISION`: policy ranked candidates and selected a profile
- `ORCHESTRATOR_DECISION`: orchestrator decision (when invoked)
- `PROFILE_SWITCH`: selected profile or fallback switch
- `CONFIG_RENDER`: config rendered for a profile
- `CONFIG_ACCEPTED`: `sing-box check -c` accepted the config
- `SINGBOX_START`: sing-box start attempt began
- `SINGBOX_STARTED`: sing-box started (includes pid)
- `SINGBOX_START_FAILED`: sing-box failed to start (classified reason)
- `PROBE_OK`: HTTP probe succeeded (routing proof)
- `PROBE_FAILED`: HTTP probe failed (classified reason)
- `CONNECTION_SUCCESS`: connection validated
- `CONNECTION_FAIL`: connection failure with reason/profile
- `AGENT_HOLD`: runtime keepalive enabled

## Streaming API (SSE)

The Inside agent runtime API is localhost-only. It exposes:

- `GET /api/events/stream`: Server-Sent Events stream
- `GET /api/events`: recent events as JSON array
- `GET /api/state`: current runtime snapshot
- `GET /api/health`: `{ "status": "ok" }`

SSE format:

```
data: {"timestamp":...,"type":"...","message":"...","metadata":{...}}

```

The server sends a backlog (up to 100) immediately on connect, then pushes new events as they occur.

## Next.js Proxy

Browsers cannot directly access a random localhost port from a hosted dashboard. The website proxies to the runtime API:

- `GET /api/proxy/events` → `/api/events/stream` (SSE)
- `GET /api/proxy/events/list` → `/api/events` (JSON)
- `GET /api/proxy/state` → `/api/state`

## Debugging Workflow

1. Open `/dashboard/runtime`
2. Use the status cards + failure list to confirm:
   - selected profile
   - recent failures (if any)
   - update timestamp
3. Use `Event Timeline` to trace the full flow:
   - profile selection → config render → sing-box start → probe → success/fail

## Privacy Rules

Events must never include:

- keys, identities, tokens, or passwords
- raw bundle contents
- full endpoint lists or full config payloads

Keep metadata small and decision-relevant:

- profile IDs (ok)
- coarse failure reason (ok)
- confidence numbers (ok)
- selected candidate lists (small, capped) (ok)

## Extending Events

When adding new decision points:

- Emit an event with a stable `type`
- Provide a short `message`
- Add minimal `metadata` required to explain “why”

Good examples:

- `POLICY_DECISION` with `candidates`, `selected`, `reason`, `confidence`
- `CONNECTION_FAIL` with `reason`, `profile`, `attempt`
