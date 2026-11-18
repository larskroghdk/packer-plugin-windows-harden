# guest/

PowerShell embedded in the plugin (`go:embed scripts/*.ps1`) and run on the Windows guest, one action per invocation. Parent: [AGENTS.md](../AGENTS.md). Constraints: [INVARIANTS.md](../INVARIANTS.md) (I8, I13, I14 apply directly).

## Local constraints

- Target Windows PowerShell 5.1 (`#Requires -Version 5.1`). Don't use PowerShell 7-only syntax: no `??`, ternaries or `-Parallel`.
- Each action reads one JSON input file and writes one JSON output file through `Write-SWResult`. Exit 0 when the action ran. Report per-setting failures in the result, and exit 1 only when the whole action failed.
- Function names use the `SW` noun prefix, and each script part is dot-sourced from `Invoke-SecureWindows.ps1`. A new `SecureWindows.<Part>.ps1` must be added to the loader list there.
- Guest output field names are the Go `json` tags in `internal/facts` and `internal/plan`. Rename them on both sides together.

## Footguns

- `Set-StrictMode -Version Latest` is on. Reading a missing property on a `ConvertFrom-Json` object throws.
- Write output as UTF-8 without a BOM. The Go side strips a BOM, but other consumers may not.
- `secedit /export` writes UTF-16 (`[Text.Encoding]::Unicode`). Read it with that encoding.
- `secedit` exit code 3 means it finished with warnings, not that it failed. The result is verified by re-inspection.
- Always use `Get-SWRegistryRoot` (the 64-bit view), never `HKLM:` provider paths, so you never touch the redirected WOW64 view.
- stdout is streamed to the Packer UI (`Write-SWLog`). Don't write secrets or large dumps there.
- In Windows PowerShell 5.1, `ConvertFrom-Json` writes a JSON array as one object. `@(... | ConvertFrom-Json)` then gives a single item that holds the whole array. Assign the result to a variable and output the variable to enumerate it (see `Read-SWInput`). pwsh doesn't behave this way, so a test that passes only under pwsh proves nothing.
- Account names are localized (`Everyone` is `Alle` on da-DK). Use SIDs in code and tests, and translate a SID when a test needs the name.
- In a comment-based help block, a line that starts with `.` followed by a word (for example `.NET returns ...`) is read as a help keyword. An unknown keyword makes PowerShell ignore the whole block without any warning. Don't start help lines with a dot, and check with `Get-Help <function>` after editing help.
- `make test-ps` runs `test/pester/` under `powershell.exe` 5.1 (`PESTER_SHELL`). The tests cover parsing, registry and the entry point on the local machine, but not secedit/auditpol writes, the guard task or WinRM. Verify those on a real guest.
