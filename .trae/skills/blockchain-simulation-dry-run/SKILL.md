---
name: "blockchain-simulation-dry-run"
description: "Runs safe fork-based dry-runs for fund-moving scripts. Invoke before any deposit/withdraw/swap/bridge automation or calldata/gas changes."
---

# Blockchain Simulation Dry Run

Use this skill to validate fund-moving flows in a forked, isolated environment before touching real assets.

## When To Invoke

- Before shipping any script that moves funds.
- When updating calldata, approvals, sequencing, slippage, or gas behavior.
- During incident review to replay failing transaction paths safely.

## Core Capabilities

- Fork-RPC scenario testing for deposit/withdraw/swap/bridge workflows.
- Approval and allowance validation before execution.
- Gas estimate and calldata-shape debugging.
- Multi-step transaction sequence validation.
- Seeded balance setup for deterministic dry-runs.

## Prerequisites

- Node.js 18+ and npm.
- RPC access for a fork/simulation endpoint (for example, Gorlami fork RPCs).
- Environment file for private keys and RPC URLs (never commit secrets).

## Safe Execution Pattern

1. Configure fork RPC and funded simulation account.
2. Validate token decimals/units and slippage bounds.
3. Run approve -> action sequence with explicit checks after each step.
4. Verify expected state delta after each transaction.
5. Fail fast on mismatched balances, wrong recipient, or abnormal gas.

## Validation Checklist

- All approvals are exact and scoped.
- Calldata target addresses and function selectors match expected contracts.
- Sequence order is deterministic and replayable.
- Balance deltas and events match expected outputs.
- Script exits non-zero on any validation failure.

