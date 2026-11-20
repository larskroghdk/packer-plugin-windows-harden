# GitHub Copilot instructions

This repository contains a Packer provisioner plugin that applies Microsoft security baselines to Windows images over WinRM while preserving the Packer session. Go owns policy decisions and orchestration; embedded PowerShell scripts detect, read and write guest state.

## Repository guidance

Read [AGENTS.md](../AGENTS.md) and [INVARIANTS.md](../INVARIANTS.md) before making changes. Invariants are hard constraints even when tests pass. Read the relevant module guidance for the files you change:

- [provisioner/AGENTS.md](../provisioner/AGENTS.md): configuration, orchestration, rollback guard and restart.
- [internal/plan/AGENTS.md](../internal/plan/AGENTS.md): comparison, WinRM protection and verification.
- [internal/baseline/AGENTS.md](../internal/baseline/AGENTS.md): baseline model, catalog and `cmd/sctimport`.
- [guest/AGENTS.md](../guest/AGENTS.md): embedded PowerShell scripts and guest tests.

## Implementation constraints

- Keep baseline selection, applicability, protection and compliance decisions in Go. Guest scripts exchange JSON files through individual PowerShell action calls.
- Route every non-compliant setting through `plan.Build` and its protection rules. Apply only `ActionApply` and `ActionGuard` items. Defer settings when the state needed to establish safety is unknown.
- Keep guarded changes registry-only, snapshot restorable state and arm rollback before applying them. Preserve WinRM probes and post-apply verification. Mark a setting `hardened` only after apply succeeds and re-inspection matches the baseline.
- Preserve strict mode: if any setting would be deferred, make no changes.
- Preserve the supported scope: WinRM, elevated 64-bit PowerShell, in-lifecycle releases with an implemented baseline, and computer configuration under HKLM. Reject domain controllers, LTSC editions, unsupported releases and unknown builds.
- Never edit `internal/baseline/data/*.json` by hand. Update `cmd/sctimport/sources.json` and regenerate from pinned Microsoft downloads with recorded SHA-256 hashes. Account for every GPO component by including it or explicitly excluding it with a reason.
- Preserve the `//go:embed` directives for baseline JSON and guest scripts.
- Regenerate `provisioner/config.hcl2spec.go` with `make generate` when a `Config` mapstructure field changes. Increment `report.SchemaVersion` for incompatible report JSON changes.

## Source documentation

Follow I19 in [INVARIANTS.md](../INVARIANTS.md). Source comments are limited to Go documentation, PowerShell comment-based help and functional directives; generated files are exempt. Put implementation explanations in Markdown.

- Document every Go function and method, including unexported and test functions, starting with its name. Include `Parameters:` for non-obvious parameters and a `Returns:` list for functions returning an error or multiple values. End with `Author: Lars Krogh` and a semantic `Version:`. Document exported types and non-test packages.
- Give every PowerShell script and function comment-based help with the required synopsis, description, parameter entries, example, outputs where applicable, and author/version notes. Place script help directly after `#Requires`, followed by two blank lines; place function help at the top of the body.
- Keep the existing author and raise the documentation version when behavior changes.

## Validation

Run `make check` before committing. It checks formatting, source documentation, PowerShell analysis, example HCL formatting, static analysis, build, vet, Go tests and Pester tests. If `make` is unavailable on Windows, run the corresponding recipes from [Makefile](../Makefile) directly. Use `make generate-check` when validating generated configuration.

Report the checks actually run and any limitations. Passing checks do not establish that orchestration or a real WinRM build works: `provisioner/` and `internal/report/` have no Go tests, and `test/integration/` is empty. PSScriptAnalyzer runs at Error severity; existing warnings are accepted.

## Task-specific procedures

- For baseline additions or refreshes, follow [update-baseline](../.agents/skills/update-baseline/SKILL.md).
- For settings that could break WinRM, follow [add-protection-rule](../.agents/skills/add-protection-rule/SKILL.md).
