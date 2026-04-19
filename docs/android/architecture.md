# Android Architecture (MVP)

## Components

- `MainActivity`: minimal UX (Connect, Disconnect, Import configuration, state/log display).
- `ShadowNetVpnService`: Android `VpnService` owner of TUN lifecycle.
- `AgentService`: foreground background runtime supervisor.
- `SingBoxController`: writes/uses config path, starts/stops/restarts sing-box.
- `Bridge`: Kotlin -> Go bridge entry points.
- `StateRepository`: persisted connection state for UI and restart resilience.
- `Logs`: bounded local-only in-memory log stream.

## Control Flow

1. UI requests connect.
2. VPN permission is requested/granted.
3. `ShadowNetVpnService` establishes TUN.
4. `AgentService` starts Go loop via `Bridge.StartAgent(config)`.
5. Go loop selects profile and renders config file.
6. `SingBoxController` starts sing-box process with rendered config.
7. `AgentService` monitors status and restarts sing-box with bounded retries.

## Go Bridge Contract

Exported in `pkg/mobilebridge`:

- `StartAgent(config string)`
- `StopAgent()`
- `ImportBundle(path string) error`
- `GetStatus() string`

The status payload is JSON and consumed by `StateRepository`.

## Safety Design

- Fail-safe fallback on bridge/orchestrator/sing-box failures.
- No remote telemetry.
- Local encrypted state via existing profile/template stores.
- Optional Pi orchestration remains non-blocking.
