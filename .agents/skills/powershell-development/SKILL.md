---
name: powershell-development
description: Implement or debug embedded Windows guest PowerShell and its Pester tests, especially JSON contracts, registry operations, native commands, rollback and restart behavior. Use for guest-script changes; PowerShell 7 alone is not a supported guest test.
---

# PowerShell development

Read [guest/AGENTS.md](../../../guest/AGENTS.md) and [INVARIANTS.md](../../../INVARIANTS.md). The guest runs elevated, 64-bit Windows PowerShell 5.1; host-side `pwsh` analysis does not change that runtime. Keep policy decisions in Go.

## Work at the guest boundary

1. Locate the action in [Invoke-SecureWindows.ps1](../../../guest/scripts/Invoke-SecureWindows.ps1), its dot-sourced implementation, and the corresponding Go `json` tags. Inspect both producers and consumers before changing a field.
2. Keep structured objects on the success pipeline. Use the existing logging and result writers; formatting cmdlets or incidental method return values must not contaminate JSON.
3. For added or changed collection fields, test zero, one and multiple items crossing the wire. Distinguish absent objects, empty arrays, empty strings and failed inspection; do not infer one from another. Account for 5.1's `ConvertFrom-Json` enumeration and strict-mode property access.
4. Use the existing native-command wrapper for `secedit` and `auditpol`. Preserve per-action exports and batched policy writes, per-setting errors versus action failures, and the documented secedit warning exit code. Check exit codes rather than assuming `$ErrorActionPreference` classifies native-process failures.
5. Dispose registry handles and other owned resources in `finally`. Use `Get-SWRegistryRoot` for the 64-bit HKLM view, the existing unsigned DWORD conversions, and SIDs for identity comparisons across localized Windows installations.

For new guest helpers, use an approved verb and the `SW` noun prefix, `[CmdletBinding()]` and parameter validation where they improve the contract. Follow I19's help requirements rather than adding inline commentary. A new script part also needs an entry in the loader and must remain embedded by `guest/guest.go`.

## Protect recovery

For guard or restart changes, read [guard.go](../../../provisioner/guard.go), [restart.go](../../../provisioner/restart.go) and [SecureWindows.Guard.ps1](../../../guest/scripts/SecureWindows.Guard.ps1). Preserve snapshot ownership, acknowledgement checks and recovery files while rollback may still need them. Test the JSON contract and synchronization without modifying the test host's system policy.

## Validate in the intended runtime

Run the affected Pester tests under `powershell.exe` 5.1, then `make test-ps`, `make pslint` and `make doccheck` as applicable. The [build workflow](../../../.github/workflows/build.yml) shows the pinned modules and separate host/guest shells. Confirm edited function help with `Get-Help`.

For actual policy writes, scheduled tasks, WinRM or restart recovery, use an authorized disposable guest and inspect the report. A `pwsh` pass, mocked task or local parser test does not establish live connectivity. Keep credentials out of source, JSON results and logs.
