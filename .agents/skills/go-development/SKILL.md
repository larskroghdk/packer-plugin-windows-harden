---
name: go-development
description: Implement or debug Go changes in this Packer plugin, including planning, orchestration, reporting and SCT import. Use for Go behavior or package-boundary changes; baseline refreshes and WinRM rules have specialized skills.
---

# Go development

Read [AGENTS.md](../../../AGENTS.md), [INVARIANTS.md](../../../INVARIANTS.md) and the owning module guide before editing. Commands run from the repository root. Documentation requirements come from I19, including unexported and test functions.

## Locate the responsibility

- Keep baseline selection and validation in `internal/baseline`, pure comparison and protection in `internal/plan`, and guest facts and lifecycle classification in `internal/facts`.
- Keep Packer configuration and execution in `provisioner`. Use its existing `packer.Communicator` boundary and `run.now` clock; add an interface only when an actual consumer needs a smaller contract.
- Keep JSON reporting in `internal/report` and baseline generation in `cmd/sctimport`. Follow [update-baseline](../update-baseline/SKILL.md) for catalog or lifecycle updates and [add-protection-rule](../add-protection-rule/SKILL.md) for connectivity-sensitive decisions.

## Implement the change

1. Trace the caller, state transition and failure path before choosing a patch. Preserve public configuration and wire contracts unless the task requires a change.
2. Keep errors actionable at the failing operation. Wrap with `%w` when callers need the underlying error; handle an error or return it rather than logging the same failure at every layer.
3. Pass the caller's context through operations that accept one, cancel derived contexts and bound retries by the remaining deadline. The SDK's `Upload` and `Download` methods have no context parameter: a command timeout alone does not prove a transfer can be canceled.
4. Keep ownership explicit for files, streams, timers and goroutines. New goroutines need a reason, a termination path and tests for cancellation; avoid background work that outlives a provisioning run.
5. Validate raw JSON or imported data where it enters the model. Preserve unknown state as unknown; replacing nil facts with permissive zero values can bypass protection rules.

Use `remote.action` for guest script operations and its JSON input contract. Do not interpolate setting data or credentials into PowerShell command text.

## Verify the behavior

Use table-driven tests for related input cases, real pure planning logic, and the existing simulated communicator for orchestration. Keep fakes in `_test.go` files. Same-package tests of unexported helpers are valid; do not export helpers or add wrappers just to satisfy a testing style.

Run focused `go test` cases while editing, then the applicable [Makefile](../../../Makefile) checks. Configuration decoding field changes require `make generate` and a review of `provisioner/config.hcl2spec.go`. Run `make generate-check` once the expected generated changes are staged or committed: its diff check compares the working file with Git's index. For concurrency changes, also run `go test -race ./...` on a supported host with CGO and the required C compiler; report an unavailable race run explicitly. Use benchmarks when a measured bottleneck motivates an optimization.

Review the final diff for changed error semantics, dropped cleanup, new dependencies and accidental edits to generated baselines. Report which behavior was tested and what still needs a real guest.
