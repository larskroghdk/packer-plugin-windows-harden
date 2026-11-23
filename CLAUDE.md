# Claude Code project instructions

@INVARIANTS.md
@.agents/architecture/overview.md
@.agents/standards/go.md
@.agents/standards/powershell.md
@.agents/standards/testing.md
@.agents/standards/security.md

Reusable workflows live only in `.agents/skills/<name>/SKILL.md`. Read the relevant file explicitly for Go development, PowerShell development, code review, releases, baseline updates or WinRM protection. Read the owning module's `AGENTS.md` before editing.

Claude-specific reviewer definitions live in `.claude/agents`. They return read-only findings to the parent agent and reference the same shared guidance. No duplicated `.claude/skills` collection is maintained. See [agent guidance](docs/agent-skills.md) for discovery limitations.
