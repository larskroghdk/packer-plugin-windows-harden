## Summary

<!-- What changes and why? -->

## Related issues

<!-- `Closes #` / `Refs #`. Larger changes should start with an issue (see CONTRIBUTING.md). -->

## Testing

- [ ] `make check` passes
- [ ] `make generate-check` passes if a `mapstructure` field changed
- [ ] New or changed behavior has tests
- [ ] Guest-script tests pass under Windows PowerShell 5.1
- [ ] Live guest run: guest OS/build and attached report, or "Not run" with reason

Simulated communicator tests do not prove live WinRM, rollback or restart recovery.

## Security and session safety

- [ ] No credentials or secrets in code, reports, logs or artifacts, or N/A
- [ ] Guest data passes through the JSON contract, not interpolated into PowerShell, or N/A
- [ ] New WinRM-sensitive settings have a protection rule that fails closed: [WinRM protection](https://github.com/larskroghdk/packer-plugin-windows-harden/blob/main/docs/winrm-protection.md), or N/A
- [ ] Guarded changes remain registry-only and snapshotted, or N/A
- [ ] Baseline data is regenerated with `cmd/sctimport`, not edited by hand, or N/A
- [ ] Actions remain pinned to commit SHAs with minimum permissions, or N/A

Security impact:

## Invariants

Affected `I<n>` from [INVARIANTS.md](https://github.com/larskroghdk/packer-plugin-windows-harden/blob/main/INVARIANTS.md), or `None`:

## Compatibility

- [ ] Report JSON change increments `report.SchemaVersion` (I17), or N/A
- [ ] Configuration change regenerates `config.hcl2spec.go` (I16) and updates [docs/configuration.md](https://github.com/larskroghdk/packer-plugin-windows-harden/blob/main/docs/configuration.md), or N/A

## Documentation

- [ ] README, docs/, examples, AGENTS.md or module guides updated where behavior changed, or N/A
- [ ] Go doc comments and PowerShell help follow I19
