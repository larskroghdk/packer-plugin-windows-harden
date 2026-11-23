# Testing requirements

[Makefile](../../Makefile) owns executable checks. Run `make check` before committing, as required by [INVARIANTS.md](../../INVARIANTS.md) I18. If make is unavailable on Windows, execute its recipe lines directly.

- Checks include formatting, documentation, Go build/vet/staticcheck, vulnerability scanning, Go tests, PowerShell analysis, example formatting and Pester tests.
- Run Pester under `powershell.exe` 5.1; host `pwsh` is used for analysis. Use pinned tools from the [build workflow](../../.github/workflows/build.yml).
- Run `make generate-check` for configuration-generation changes and a snapshot for packaging changes. Add race testing for concurrency changes when supported.
- Test observable failures with real planning/parsing logic and existing communicator seams. Isolate tests from the host's system policy.
- Local and simulated tests do not prove live WinRM, scheduled-task rollback or restart recovery. Use an authorized disposable guest for those checks.

Report actual results and limitations. Use [code-review](../skills/code-review/SKILL.md) for review and test-quality work.
