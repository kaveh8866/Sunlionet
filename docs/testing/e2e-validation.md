# End-to-End (E2E) Validation: Real sing-box Runtime

This document defines what “working” means for SunLionet Inside and how to prove real traffic is flowing through a real `sing-box` process (not just config rendering).

## Real Runtime Success Criteria

A run is successful only if all of the following are true:

- `[sing-box] started`: `sing-box run` starts and stays alive long enough to serve requests.
- `[config] accepted`: `sing-box check -c config.json` succeeds (no parse errors).
- `[probe] HTTP success via proxy`: an HTTP GET to `https://example.com` succeeds through the local proxy inbound exposed by `sing-box`.

The Inside runtime writes a machine-readable state file:

- `state.json.status` must be `running`
- `state.json.probe.status` must be `ok`
- `state.json.attempts[]` shows each attempted profile with probe status and failure reason.

## Linux: Manual E2E Run

Prerequisites:

- `sing-box` installed and in `PATH`, or pass `--sing-box-bin /path/to/sing-box`
- A real bundle that contains at least one working profile (Reality / TUIC / etc.)

Run:

```bash
go run ./cmd/inside \
  --mode=real \
  --state-dir ./.tmp/state \
  --import ./path/to/bundle.snb.json \
  --master-key 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
  --trusted-signer-pub-b64url "$(cat ./trusted_signer_pub.b64url)" \
  --age-identity "$(cat ./inside.agekey)" \
  --probe-url https://example.com \
  --probe-proxy-addr 127.0.0.1:18080 \
  --max-attempts 3 \
  --verbose
```

Or use the helper runner:

```bash
export SUNLIONET_E2E_BUNDLE=/absolute/path/to/bundle.snb.json
export SUNLIONET_E2E_TRUSTED_SIGNER_PUB_B64URL="$(cat /absolute/path/to/trusted_signer_pub.b64url)"
export SUNLIONET_E2E_AGE_IDENTITY="$(cat /absolute/path/to/inside.agekey)"
export SUNLIONET_E2E_MASTER_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
bash ./tests/e2e/linux/run_real.sh
```

What to expect:

- Logs include `[sing-box] started pid=...` and `[probe] ok ...`.
- `./.tmp/state/state.json` includes `status=running` and `probe.status=ok`.

If probe fails:

- The run fails with `runtime failed after N attempts`.
- `state.json.probe.reason` classifies the failure:
  - `CONFIG_ERROR`
  - `BINARY_MISSING`
  - `DNS_FAILURE`
  - `NETWORK_BLOCKED`
  - `TIMEOUT`
  - `UNKNOWN`

## Linux: Optional Go E2E Test

This repository includes an opt-in E2E test that only runs on Linux and only when explicitly enabled.

Enable:

```bash
export SUNLIONET_E2E=1
export SUNLIONET_E2E_BUNDLE=/absolute/path/to/bundle.snb.json
export SUNLIONET_E2E_TRUSTED_SIGNER_PUB_B64URL="$(cat /absolute/path/to/trusted_signer_pub.b64url)"
export SUNLIONET_E2E_AGE_IDENTITY="$(cat /absolute/path/to/inside.agekey)"
export SUNLIONET_E2E_MASTER_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
go test ./tests/e2e/linux -run TestInside_RealSingBox_HTTPProbe -v
```

Notes:

- The test skips if `sing-box` is not in `PATH`.
- The test requires a real working bundle; the repository’s sample/integration bundles are not guaranteed to be routable.

## Android: End-to-End Validation

The Android runtime validates and proves routing by:

- Extracting and verifying the `sing-box` binary checksum from assets.
- Running `sing-box check -c <config>` before starting.
- Starting `sing-box run -c <config>`.
- Running an HTTP probe that uses the VPN network transport (not the default network).

Expected device logs/state:

- `sing-box check ok`
- `[connection] testing...`
- `[connection] success http=...` (only when VPN is active and `sing-box` is functioning)

If it fails:

- `[connection] failed: DNS_FAILURE|TIMEOUT|NETWORK_BLOCKED|UNKNOWN`

## Troubleshooting

- `BINARY_MISSING`: install `sing-box` (Linux) or ensure APK assets include the ABI binary (Android).
- `CONFIG_ERROR`: run `sing-box check -c config.json` and inspect stderr logs.
- `DNS_FAILURE`: outbound DNS may be blocked; verify DoH reachability and detour routing.
- `NETWORK_BLOCKED`: upstream endpoint is unreachable/filtered; try a different profile.
- `TIMEOUT`: endpoint reachable but handshake/route is stuck; inspect sing-box stderr logs and profile parameters.
