# Runtime Modes

SunLionet runs in one of two explicit modes:

- `real`: Production runtime. Only real implementations are allowed.
- `simulation`: Test/simulation runtime. Simulated implementations are allowed.

## Why This Exists

In censorship environments, hidden mock logic is dangerous. It can:

- Produce false-positive “healthy” signals
- Mask missing dependencies
- Alter timing/behavior in ways that make detection and recovery unreliable

This project enforces a strict separation so production behavior is deterministic and auditable.

## How To Run

### Real Mode (Default)

```bash
go run ./cmd/inside --mode=real --render-only --state-dir=/path/to/state --master-key=YOUR_32_BYTE_KEY
```

### Simulation Mode

```bash
go run ./cmd/inside --mode=simulation --render-only --state-dir=/path/to/state --master-key=YOUR_32_BYTE_KEY
```

## Guarantees

- Real mode never selects simulation implementations.
- Simulation mode must be explicitly requested.
- Mode selection happens only in the `cmd/inside` composition root.
