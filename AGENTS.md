# Codex project instructions

This Go Packer plugin applies Microsoft security baselines over WinRM. Follow [INVARIANTS.md](INVARIANTS.md) and the owning module's `AGENTS.md` before editing.

## Shared project knowledge

- [Architecture and module map](.agents/architecture/overview.md)
- [Go standards](.agents/standards/go.md)
- [PowerShell standards](.agents/standards/powershell.md)
- [Testing requirements](.agents/standards/testing.md)
- [Security practices](.agents/standards/security.md)

Run `make check` before a commit; if make is unavailable, run the [Makefile](Makefile) recipes directly. Go and simulated communicator tests do not establish live WinRM behavior.

## Skills

- [go-development](.agents/skills/go-development/SKILL.md): Go implementation, package boundaries, context and resource ownership.
- [powershell-development](.agents/skills/powershell-development/SKILL.md): guest scripts, JSON contracts and Windows PowerShell 5.1 testing.
- [code-review](.agents/skills/code-review/SKILL.md): requested review, security audit, refactoring and test quality.
- [release-plugin](.agents/skills/release-plugin/SKILL.md): CI, dependency changes, packaging and authorized release publication.
- [update-baseline](.agents/skills/update-baseline/SKILL.md): add or refresh a Microsoft baseline.
- [add-protection-rule](.agents/skills/add-protection-rule/SKILL.md): add a WinRM protection rule for a setting that could break the Packer session.

Read only the workflows relevant to the task. `.agents/skills` is the single source of reusable skill instructions, not a Codex configuration directory. [Agent guidance](docs/agent-skills.md) documents ownership and discovery. Skill activation grants no additional publication or guest-operation authority.
