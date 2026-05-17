# Android Memory and VPN Boundary Hardening

This note documents the hardened JVM/Go boundary and Android VPN leak policy.

## Memory Boundary

The Android bridge now configures the Go runtime once at startup:

- `GOGC` equivalent via `debug.SetGCPercent(75)`.
- `GOMAXPROCS` capped to two mobile worker threads.
- Optional Go heap memory limit, currently 96 MiB from the Android wrapper.

High-frequency bridge calls avoid reflective method lookup churn by caching
`Method` handles in `Bridge.kt`. Status polling uses `GetStatusBytes()` so the
gomobile boundary returns raw UTF-8 bytes instead of repeatedly constructing
JVM strings from Go strings. Startup config and onboarding config calls use byte
array entry points (`StartAgentBytes`, `ImportOnboardingURIWithConfigBytes`) to
keep the boundary shape explicit and short-lived.

No unsafe native memory is retained across calls. The Go wrapper does not expose
C pointers, global byte slices, or direct references into JVM-owned arrays.
Every byte slice copied through gomobile is consumed synchronously and released
to the Go garbage collector after the call returns.

## Profiling Hooks

`RuntimeMemoryStatsJSON()` exposes coarse Go allocator counters for in-app
debug panels. `StartLocalPprof(addr)` starts standard Go pprof handlers only on
loopback addresses (`127.0.0.1`, `localhost`, or `::1`), rejecting public
listeners.

## VPN Fail-Closed Policy

The Android `VpnService` no longer advertises public DNS resolvers such as
`1.1.1.1` to the OS. It installs tunnel-local DNS servers:

- IPv4 DNS: `10.0.0.1`
- IPv6 DNS: `fd00:736c:6e::1`

The service routes both `0.0.0.0/0` and `::/0` into the VPN and explicitly
allows both address families. During network switches or proxy-core failures,
the VPN enters a `HOLDING` state and keeps the TUN interface established. If
the packet processor is unavailable, traffic is blackholed locally instead of
falling back to ambient Wi-Fi or cellular routes.

`setUnderlyingNetworks(emptyArray())` prevents Android from advertising an
underlying network for bypass. On Android Q and later, the service also marks
the VPN non-metered and requests blocking mode on the file descriptor.

## Watchdog

The VPN service registers a `ConnectivityManager.NetworkCallback`. Network
loss or replacement transitions the service into `HOLDING`; network availability
asks `AgentService` to restart the proxy core while the TUN route remains in
place. This keeps route ownership stable across Wi-Fi/cellular changes.
