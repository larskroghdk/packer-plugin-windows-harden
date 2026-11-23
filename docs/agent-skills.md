# Agent guidance

How the instruction files are laid out, how each coding agent finds them, and how to check them after a change.

## Layout

| File | Owns | Loaded by |
| --- | --- | --- |
| [AGENTS.md](../AGENTS.md) | Module map, run flow, Go/PowerShell/testing/security rules, skill index | Codex natively; Claude Code through `@AGENTS.md` in `CLAUDE.md`; Copilot through a link |
| [INVARIANTS.md](../INVARIANTS.md) | Hard constraints (`I<n>`) | Claude Code through `@INVARIANTS.md`; others read it because `AGENTS.md` requires it |
| Module `AGENTS.md` files | Local constraints and footguns | Codex when working in that directory; others on request |
| `.agents/skills/<name>/SKILL.md` | Task procedures | Codex as repository skills; others read the file explicitly |
| [CLAUDE.md](../CLAUDE.md) | Imports the two root files and adds Claude-only notes | Claude Code |
| `.claude/agents/*.md` | Three read-only reviewer subagents | Claude Code |
| [.github/copilot-instructions.md](../.github/copilot-instructions.md) | Points Copilot at `AGENTS.md` | GitHub Copilot |
| [Makefile](../Makefile) | Every executable check | Everyone |

Each rule has one home. Don't copy rules into a skill or a tool-specific file. Link to them instead.

## Discovery

- **Codex** reads `AGENTS.md` from the repository root down to the working directory, and lists `.agents/skills` as repository-scope skills.
- **Claude Code** loads `CLAUDE.md` and its imports. Its native skill directory is `.claude/skills`. This repository doesn't keep one, because a second copy of each skill would drift. Claude therefore reads skills as files and doesn't offer them as `/` commands. The reviewers in `.claude/agents` use Claude's [subagent format](https://code.claude.com/docs/en/sub-agents).
- **Other agents** get no native skill discovery. They should follow the links in `AGENTS.md`.

No MCP servers, hooks or permission settings are configured for the project.

## Checking a change

- Each skill needs `name` (equal to its folder name) and `description` frontmatter. Its description should say when to use it and should not overlap the other skills.
- Every local Markdown link and every `@` import in `CLAUDE.md` must resolve.
- Check Codex discovery without starting a model turn: send `initialize` and then `skills/list` with the repository as `cwds` to `codex app-server`. All six skills should come back with `scope: repo` and no errors.
- In Claude Code, the three reviewers should appear in `/agents`.

Passing these checks shows that the files are found. It doesn't show that a model will pick or follow the right skill.
