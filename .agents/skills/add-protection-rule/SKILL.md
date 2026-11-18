---
name: add-protection-rule
description: Add or change a WinRM protection rule in internal/plan/protect.go for a baseline setting that could lock Packer out of the guest (authentication, transport, firewall, UAC, logon rights). Use when a build loses WinRM after hardening or a new baseline introduces such a setting.
---

# Add a protection rule

Constraints: INVARIANTS.md I6–I9. Module context: `internal/plan/AGENTS.md`.

## 1. Choose the action

| Situation | Action |
| --- | --- |
| The guest facts can tell whether the setting breaks the current session | `ActionDefer` when it would break it; otherwise return `nil` (apply) |
| The effect depends on the Packer client and can't be decided from the guest (e.g. NTLM levels) | `ActionGuard`. It must be a registry setting, because rollback restores registry values only. |
| The needed facts are missing (`nil` sections in `facts.Facts`) | `ActionDefer`. Rules fail closed. |

## 2. Add facts if you need them

If the rule needs guest data that isn't collected yet:
1. Collect it in `guest/scripts/SecureWindows.Facts.ps1` (`Get-SWSystemFacts`), and keep the section nullable on failure.
2. Add the field to `internal/facts/facts.go`. The `json` tag must match the PowerShell property name.
3. Add it to `internal/facts/testdata/ws2025-core.json` and assert it in `TestParseSample`.
4. If losing the capability after a change should fail the run, add the comparison to `WinRMRegressions` (`internal/facts/connectivity.go`).

## 3. Write the rule

- Registry settings: add a lower-case `path\name` constant and a `case` in `registryRule`. User rights: add a `case` in `userRightRule`.
- Use `deferred(...)` with a reason that names the setting's effect on the Packer session. The reason appears verbatim in the report.
- UAC-related values go through `uacModel`, not a standalone case.

## 4. Test

In `internal/plan/plan_test.go`, add table cases that cover the session state that defers, the one that applies, and missing facts. Then run `make check`.

## 5. Verify on a guest

Unit tests don't exercise WinRM. Run a real Packer build against an affected OS, first with `dry_run = true` to see the decision in the report, then without it.
