# PowerShell standards

Guest scripts target elevated, 64-bit Windows PowerShell 5.1. Read [guest/AGENTS.md](../../guest/AGENTS.md) and [INVARIANTS.md](../../INVARIANTS.md). Host PowerShell 7 analysis does not establish guest compatibility.

- Keep selection, protection and compliance decisions in Go. Guest actions detect, inspect and apply operations.
- Preserve JSON contracts and keep incidental output off the success pipeline. Test absent, empty, singleton and multiple-item collections at the Go boundary.
- Check native exit codes, dispose handles in `finally` and reuse registry and native-command helpers.
- Follow I19 for comment-based help; use approved verbs and the `SW` noun prefix for new helpers.

Use [powershell-development](../skills/powershell-development/SKILL.md) for implementation and [testing](testing.md) for runtime verification.
