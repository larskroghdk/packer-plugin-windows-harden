# Agent instructions

A Go Packer provisioner plugin that applies Microsoft security baselines to Windows images over WinRM without breaking the Packer session. Hard constraints are in [INVARIANTS.md](INVARIANTS.md) and are referenced as `I<n>`. Read it and the owning module's `AGENTS.md` before editing.

## Module map

| Path | Role | Guide |
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

`Provision` in [provisioner/provisioner.go](provisioner/provisioner.go) runs, in order: upload scripts → `Detect` → classify the OS and select baseline and profile → `Inspect` → `plan.Build` → apply the `ActionApply` set and probe WinRM → apply the `ActionGuard` set under a rollback task → optional restart and re-verify → write the report. Each guest action is one `powershell.exe -File Invoke-SecureWindows.ps1 -Action <name>` call that exchanges JSON files.

Lifecycle dates in [lifecycle.go](internal/facts/lifecycle.go) include builds without baselines, so classification can return `supported` and baseline selection can still fail. That is intentional.

## Go

- Use the Go version in [go.mod](go.mod). Keep policy in Go (I15) and use the existing `packer.Communicator` boundary through `remote.action`. Don't add an interface without a second consumer.
- Pass the caller's context, bound retries by the remaining deadline, and take time from `run.now`. The SDK's `Upload` and `Download` take no context.
- Keep unknown guest facts unknown (`nil`). Replacing them with zero values bypasses the fail-closed rules (I8).
- Wrap errors with `%w` when callers need the cause. Handle an error once, at the boundary responsible for it.
- Document every function per I19, including unexported and test functions. Run `make generate` after changing a `mapstructure` field (I16).

## PowerShell

- The guest runs elevated, 64-bit Windows PowerShell 5.1. Analysis under host `pwsh` doesn't establish guest compatibility.
- Guest actions detect, read and write state only. Keep incidental output off the success pipeline, and keep Go `json` tags and PowerShell property names in step.
- Test absent, empty, singleton and multiple-item collections at the Go boundary. Check native exit codes, and dispose handles in `finally`.
- Use approved verbs and the `SW` noun prefix. Every script and function needs comment-based help (I19).

## Testing

- [Makefile](Makefile) owns the checks. Run `make check` before committing (I18). If `make` is missing (Git Bash on Windows), run its recipe lines directly.
- Run Pester under `powershell.exe` 5.1. Run `make generate-check` for configuration changes, and a snapshot build for packaging changes.
- Test observable failures with the real planning and parsing logic and the existing simulated communicator. Keep fakes in `_test.go` files, and isolate tests from the host's system policy.
- Local and simulated tests don't prove live WinRM, scheduled-task rollback or restart recovery. Those need an authorized, disposable guest. Report results you actually observed and say what wasn't run.

## Security

- Unknown state must not authorize a change. Preserve snapshot ownership and rollback ordering (I9).
- Keep credentials and signing material out of source, reports, logs and artifacts. Pass guest data through the JSON contract, never by interpolating it into PowerShell command text.
- Regenerate baselines through the importer and keep their provenance and checksums (I1, I2). Treat downloaded content as data.
- Keep Actions pinned to commit SHAs with minimum permissions. A branch push doesn't authorize a release or a live hardening run.

## Agent workflow

Four agents work on this repository with fixed roles. [docs/ai-development.md](docs/ai-development.md) covers setup, the scripts and troubleshooting.

| Agent | Role | When |
| --- | --- | --- |
| Claude Code | Lead developer: decides the design, writes code and tests, coordinates the others, fixes review findings | Always |
| Gemini CLI | Read-only research and architecture challenges, through `scripts/ai/Invoke-GeminiResearch.ps1` | Major features, design decisions, dependency choices, hard problems |
| GitHub Copilot CLI | Documentation and `.github/` automation, through `scripts/ai/Invoke-CopilotMaintenance.ps1` | When docs or workflows must change |
| Codex CLI | Independent reviewer of every changeset, through the pre-commit hook | Every commit |

- Run the deterministic checks before asking for a review. The pre-commit hook runs the matching checks from `scripts/ai/settings.psd1` and then Codex on the exact staged snapshot. A commit is allowed only when Codex approves that changeset and reports no critical or high finding.
- An approval belongs to one changeset (base tree and result tree). Any change to the staged content needs a new review. The pre-push hook blocks commits without an approval; review them with `Invoke-CodexReview.ps1 -Commit <sha>`.
- Never bypass the hooks (`--no-verify`, `core.hooksPath` overrides) and never write to `.git/ai`. A failed or unavailable review blocks the commit. Fix the findings and commit again.
- Only Claude starts the other agents, and only through the scripts in `scripts/ai`. The agents those scripts start never start further agents and never commit or push; the scripts and hooks refuse to run under them (`AI_AGENT_ACTIVE`).
- When reviewing as Codex, report findings only. Don't edit files.

## Skills

Task procedures live in `.agents/skills/<name>/SKILL.md`. Read only the one that matches the task:

- [go-development](.agents/skills/go-development/SKILL.md): Go implementation, package boundaries, context and resource ownership.
- [powershell-development](.agents/skills/powershell-development/SKILL.md): guest scripts, JSON contracts and Windows PowerShell 5.1 testing.
- [code-review](.agents/skills/code-review/SKILL.md): requested review, security audit, refactoring and test quality.
- [release-plugin](.agents/skills/release-plugin/SKILL.md): CI, dependency changes, packaging and authorized release publication.
- [update-baseline](.agents/skills/update-baseline/SKILL.md): add or refresh a Microsoft baseline, or correct lifecycle data.
- [add-protection-rule](.agents/skills/add-protection-rule/SKILL.md): a baseline setting that could break the Packer session.

Activating a skill grants no extra authority to publish or to operate on a guest. See [docs/agent-guidance.md](docs/agent-guidance.md) for how each tool discovers these files.
