# Linux Smoke Test

This smoke test validates the Linux MVP path without internet dependency.

## What it covers

- temp state initialization
- import from local sample bundle
- signature verification + decrypt
- profile selection with deterministic ranking
- config render to disk
- launch behavior via fake `sing-box` binary in tests
- clean actionable failure when `sing-box` is missing

## Run only the Linux MVP integration smoke tests

```bash
go test ./tests/integration -run TestInsideLinuxMVP -v
```

## Run full project tests

```bash
go test ./...
```

## Expected behavior

- `TestInsideLinuxMVP_Smoke_GoRun` passes:
  - writes valid runtime config JSON
  - writes `state.json` with selected profile and fallback candidates
  - starts fake `sing-box` and records PID
- `TestInsideLinuxMVP_MissingSingBox_FailsCleanly` passes:
  - exits non-zero
  - includes actionable message containing `sing-box binary not found` and `set --sing-box-bin`

## Notes

- Fake `sing-box` is used only inside tests.
- Production runtime path uses real process control in `pkg/sbctl`.
