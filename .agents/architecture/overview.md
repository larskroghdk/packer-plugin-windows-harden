# Architecture overview

A Packer provisioner plugin that applies Microsoft security baselines to Windows images over WinRM without breaking the Packer session. Hard constraints are in [INVARIANTS.md](../../INVARIANTS.md); this file covers how to work here.

## Module map

| Path | Role | Context |
| --- | --- | --- |
| `main.go` | Plugin entry point; holds `version` | — |
| `provisioner/` | Packer config, run orchestration, guard, restart | [provisioner/AGENTS.md](../../provisioner/AGENTS.md) |
| `internal/plan/` | Compare state to baseline, WinRM protection rules, verify | [internal/plan/AGENTS.md](../../internal/plan/AGENTS.md) |
| `internal/baseline/` | Baseline model, embedded catalog, selection | [internal/baseline/AGENTS.md](../../internal/baseline/AGENTS.md) |
| `cmd/sctimport/` | Generates baseline JSON from Microsoft SCT zips | [internal/baseline/AGENTS.md](../../internal/baseline/AGENTS.md) |
| `cmd/doccheck/` | Lint for the Go documentation rules of I19 | — |
| `internal/facts/` | Guest facts, OS lifecycle classification, WinRM regression checks | — |
| `internal/report/` | JSON report and console summary | — |
| `guest/` | Embedded PowerShell run on the guest | [guest/AGENTS.md](../../guest/AGENTS.md) |

## Run flow

`Provision` in [provisioner/provisioner.go](../../provisioner/provisioner.go) runs these steps in order: upload scripts → `Detect` → classify OS and select baseline/profile → `Inspect` → `plan.Build` → apply the `ActionApply` set and probe WinRM → apply the `ActionGuard` set under a rollback task → optional restart and re-verify → write the report. Each guest action is one `powershell.exe -File Invoke-SecureWindows.ps1 -Action <name>` call that exchanges JSON files.

Lifecycle dates in [lifecycle.go](../../internal/facts/lifecycle.go) include builds without baselines. Classification can return `supported` and baseline selection can then fail; that is intentional.
