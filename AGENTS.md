# AGENTS.md

A Packer provisioner plugin that applies Microsoft security baselines to Windows images over WinRM without breaking the Packer session. Hard constraints are in [INVARIANTS.md](INVARIANTS.md); this file covers how to work here.

## Module map

| Path | Role | Context |
| --- | --- | --- |
| `main.go` | Plugin entry point; holds `version` | — |
| `provisioner/` | Packer config, run orchestration, guard, restart | [provisioner/AGENTS.md](provisioner/AGENTS.md) |
| `internal/plan/` | Compare state to baseline, WinRM protection rules, verify | [internal/plan/AGENTS.md](internal/plan/AGENTS.md) |
| `internal/baseline/` | Baseline model, embedded catalog, selection | [internal/baseline/AGENTS.md](internal/baseline/AGENTS.md) |
| `cmd/sctimport/` | Generates baseline JSON from Microsoft SCT zips | [internal/baseline/AGENTS.md](internal/baseline/AGENTS.md) |
| `cmd/doccheck/` | Lint for the Go documentation rules of I19 | — |
| `internal/facts/` | Guest facts, OS lifecycle classification, WinRM regression checks | — |
| `internal/report/` | JSON report and console summary | — |
| `guest/` | Embedded PowerShell run on the guest | [guest/AGENTS.md](guest/AGENTS.md) |

## Run flow

`Provision` in [provisioner/provisioner.go](provisioner/provisioner.go) runs these steps in order: upload scripts → `Detect` → classify OS and select baseline/profile → `Inspect` → `plan.Build` → apply the `ActionApply` set and probe WinRM → apply the `ActionGuard` set under a rollback task → optional restart and re-verify → write the report. Each guest action is one `powershell.exe -File Invoke-SecureWindows.ps1 -Action <name>` call that exchanges JSON files.

## Commands

| Command | Does |
| --- | --- |
| `make check` | lint + typecheck + test (required before commit) |
| `make generate` | Regenerate `provisioner/config.hcl2spec.go` |
| `make baselines DOWNLOADS=<dir>` | Regenerate `internal/baseline/data/*.json` |
| `make build` | Build the plugin binary |

## Repo-wide footguns

- `make` is not on the default Windows/Git Bash PATH. If it's missing, run the recipe lines from the [Makefile](Makefile) directly.
- Go tests cover planning, reports and selected orchestration paths through a simulated communicator. They do not cover a real WinRM build. Guest-script test coverage is described in [guest/AGENTS.md](guest/AGENTS.md).
- PSScriptAnalyzer runs at `Error` severity. Existing warnings, such as missing `ShouldProcess`, are known and accepted.
- Lifecycle dates in [internal/facts/lifecycle.go](internal/facts/lifecycle.go) also cover builds that have no baseline (Server 2016, Windows 10, Windows 11 22H2/23H2). Those builds classify as `supported` and then fail at baseline selection. That is intended.

## Skills

- [update-baseline](.agents/skills/update-baseline/SKILL.md): add or refresh a Microsoft baseline.
- [add-protection-rule](.agents/skills/add-protection-rule/SKILL.md): add a WinRM protection rule for a setting that could break the Packer session.
