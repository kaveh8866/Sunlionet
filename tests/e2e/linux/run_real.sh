#!/usr/bin/env bash
set -euo pipefail

: "${SHADOWNET_E2E_BUNDLE:?set SHADOWNET_E2E_BUNDLE}"
: "${SHADOWNET_E2E_TRUSTED_SIGNER_PUB_B64URL:?set SHADOWNET_E2E_TRUSTED_SIGNER_PUB_B64URL}"
: "${SHADOWNET_E2E_AGE_IDENTITY:?set SHADOWNET_E2E_AGE_IDENTITY}"
: "${SHADOWNET_E2E_MASTER_KEY:?set SHADOWNET_E2E_MASTER_KEY}"

STATE_DIR="${SHADOWNET_E2E_STATE_DIR:-./.tmp/state}"
SINGBOX_BIN="${SHADOWNET_E2E_SINGBOX_BIN:-}"
PROBE_URL="${SHADOWNET_E2E_PROBE_URL:-https://example.com}"
PROBE_PROXY_ADDR="${SHADOWNET_E2E_PROBE_PROXY_ADDR:-127.0.0.1:18080}"
MAX_ATTEMPTS="${SHADOWNET_E2E_MAX_ATTEMPTS:-3}"

args=(
  "run" "./cmd/inside"
  "--mode=real"
  "--state-dir" "${STATE_DIR}"
  "--import" "${SHADOWNET_E2E_BUNDLE}"
  "--master-key" "${SHADOWNET_E2E_MASTER_KEY}"
  "--trusted-signer-pub-b64url" "${SHADOWNET_E2E_TRUSTED_SIGNER_PUB_B64URL}"
  "--age-identity" "${SHADOWNET_E2E_AGE_IDENTITY}"
  "--probe-url" "${PROBE_URL}"
  "--probe-proxy-addr" "${PROBE_PROXY_ADDR}"
  "--max-attempts" "${MAX_ATTEMPTS}"
  "--verbose"
)

if [[ -n "${SINGBOX_BIN}" ]]; then
  args+=("--sing-box-bin" "${SINGBOX_BIN}")
fi

go "${args[@]}"
