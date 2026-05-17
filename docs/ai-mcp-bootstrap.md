# SunLionet AI MCP Bootstrap

This project-local bootstrap defines only lightweight interfaces for auditing,
securing, testing, and refactoring the SunLionet core logic.

## MCP Toolsets

- `sequential_thinking`: configured through `npx -y @modelcontextprotocol/server-memory`.
  Use it as a small reasoning scratchpad for multi-step validation of
  cryptographic, failover, and state-machine changes.
- `filesystem`: configured through `npx -y @modelcontextprotocol/server-filesystem`
  with access scoped to `C:\Users\Kaveh\Desktop\Iran-Agent-Vpn`.

The repository config is stored at `.mcp.json`. A client that supports
project-level MCP discovery can load it directly.

## Local Audit Bindings

Run these from the repository root:

```powershell
.\scripts\audit_go_security.ps1
.\scripts\audit_chaos_mock.ps1
.\scripts\audit_crypto_state.ps1
.\scripts\audit_sunlionet_core.ps1
```

`audit_go_security.ps1` uses installed `gosec` and `govulncheck` binaries when
available, and falls back to `go run ...@latest` bindings.

`audit_chaos_mock.ps1` exercises the existing degradation, blackout, and
resilience simulations without requiring a full OS/container chaos platform.

`audit_crypto_state.ps1` focuses on bundle verification, message crypto,
identity, ledger, ledgersync, mesh, and release manifest validation.

## Active Codex Session Note

Codex cannot hot-load new MCP tool-call definitions into the running client from
inside a conversation. In this session, secure code mutation is available through
the existing workspace tools, and structural validation is handled natively by
Codex reasoning. The `.mcp.json` file is ready for any MCP-aware client reload.
