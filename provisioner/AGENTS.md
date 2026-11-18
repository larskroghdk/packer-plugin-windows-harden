# provisioner/

Packer-facing layer: config decoding, run orchestration, the connectivity guard and restart handling. Parent: [AGENTS.md](../AGENTS.md). Constraints: [INVARIANTS.md](../INVARIANTS.md) (I6, I8, I10, I11, I15 apply directly).

## Local constraints

- Validate config options in `Config.prepare` ([config.go](config.go)), and return all errors with `packer.MultiErrorAppend`. Don't fail on the first one.
- Every guest call goes through `remote.action` ([remote.go](remote.go)). Don't build PowerShell command lines anywhere else.
- Take the time from `run.now`, not `time.Now`. The guard and probe deadlines are computed from it.

## Footguns

- Adding or renaming a `Config` field without running `make generate` leaves `config.hcl2spec.go` stale. HCL2 templates then reject the option, but JSON templates still accept it.
- `fail_on_error` is a `config.Trilean`. Unset means true (`failOnError` is `!False()`).
- The guard timings in [guard.go](guard.go) depend on each other: `guardDelay` (3m) has to be longer than `guardProbeWindow` (90s) plus the apply time. If you shorten it, the guest task can roll back a working change before it's disarmed. That case is handled, but the setting is reported as deferred.
- After a restart, the guest working directory under `C:\Windows\Temp` may be gone, so `restartAndReverify` calls `remote.install` again. Any new post-restart step needs the scripts reinstalled first.
- The report is still written when `execute` fails. A report-write error is returned only when the run itself succeeded.
