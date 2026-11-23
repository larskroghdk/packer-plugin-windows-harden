# Agent skills

The six workflows indexed in [AGENTS.md](../AGENTS.md) extend the repository's existing guidance. `INVARIANTS.md` owns hard constraints, module `AGENTS.md` files own local context, and [Makefile](../Makefile) owns executable checks. Skills explain how to apply those sources to a task.

## Assessment and gaps

Reviewed all 20 skill entrypoints in [computerlovetech/umbod at commit 40c9c04](https://github.com/computerlovetech/umbod/tree/40c9c04dd0b726ca15e65e2509880868926695c2/skills), plus the supporting architecture, testing, release and authoring material needed for the decisions below. The nested `write-skill/references/templates/SKILL.md` is a template, not a twenty-first skill. Upstream is [MIT licensed](https://github.com/computerlovetech/umbod/blob/40c9c04dd0b726ca15e65e2509880868926695c2/LICENSE). The local workflows are newly written for this plugin; upstream code, scripts and templates are not vendored or executed.

The existing collection covered only baseline imports and protection rules. It lacked reusable guidance for Go ownership/cancellation, the Go–PowerShell wire contract, runtime-specific testing, review and plugin releases. Claude had shared root instructions but no project skill entrypoints. Copilot repeated many of the invariants. The baseline skill also required unrelated files to change on every update and treated lifecycle corrections like package refreshes.

The collection now keeps architecture and integration inside the language workflows, testing and security review inside `code-review`, and dependency/CI work inside `release-plugin`. Separate architect, documentation, performance and Windows-automation personas would mostly repeat the same sources.

## External skill decisions

Paths below are relative to the pinned upstream `skills/` tree. Adapt means retaining a useful idea in a project-specific workflow; no skill is installed unchanged.

| External skill | Decision | Local treatment |
| --- | --- | --- |
| `architecture/api-client-architecture` | Adapt | Validate JSON boundaries in the language workflows. Reject Zod, route classes and a composite HTTP client: the existing Packer communicator is the transport boundary. |
| `architecture/choose-design-pattern` | Merge | Keep evidence-based, smaller refactors in Go development and review. Omit the OO catalog and mandatory full decision-tree read. |
| `architecture/ports-adapters-io` | Adapt | Preserve the pure plan package and reuse `packer.Communicator`. Keep fakes in tests; avoid requiring a new port and in-memory adapter for every I/O operation. |
| `architecture/write-adr` | Reject | There is no established ADR collection to maintain. Explain consequential changes in existing documentation and review descriptions; create a record when requested. |
| `development/improve-testability` | Merge | Keep focused behavioral tests and resource ownership in `code-review`. Drop fixed three-file batches, automatic stops and the blanket ban on testing unexported Go helpers. |
| `development/release-umbod` | Adapt | Use candidate commits, immutable tags, observed assets and bounded recovery in `release-plugin`. Omit Umbod versions, PyPI, images, Helm, production branches and Cloudflare. |
| `development/test-writing` | Adapt | Use Go tests, Pester 5.1, entry-script tests and disposable WinRM guests. Do not install pytest, Vitest, Kind or a browser runner. |
| `development/write-module-readme` | Merge | Retain useful links and progressive disclosure in existing module guides. Reject an OKF conversion, metadata per document and four additional Python README checks. |
| `development/write-skill` | Adapt | Keep task-specific triggers, standard frontmatter and real-resource validation. Reject fixed section/item counts and the arbitrary 3,000-character gate; no separate authoring skill or copied validator is needed. |
| `frontend/frontend-package-manager` | Reject | Bun/uv rules do not govern Go modules or the pinned PowerShell modules in this project. |
| `frontend/reusable-components` | Reject | No frontend component tree, component index or associated ADR exists here. |
| `frontend/svelte/create-state-class` | Reject | Svelte reactive state classes have no project consumer and would encourage inappropriate Go abstractions. |
| `frontend/svelte/effect-usage-guard` | Reject | No Svelte effects exist. Go context and resource ownership are covered by the Go workflow. |
| `frontend/svelte/svelte-docs-best-practices` | Reject | No Svelte/SvelteKit code or Node toolchain is present. |
| `integrations/run-diagnostics` | Reject | Depends on Umbod MCP playbooks and diagnostic submission; no corresponding project integration exists. |
| `integrations/umbod-agent-behaviour` | Reject | External obstacle telemetry is unrelated to building a Packer plugin and must not become a repository-wide reporting obligation. |
| `integrations/umbod-connected-apps` | Reject | Umbod-specific tools and access rules are outside this repository's runtime and development dependencies. |
| `other/agent-browser-ui-check` | Reject | The integration boundary is WinRM and a Windows guest, with no UI to validate in a browser. |
| `python/fastapi-dependency-lifetime` | Reject | FastAPI providers and application lifespans do not match the Packer SDK. Go resource ownership is addressed directly. |
| `python/python-domain-model-none-design` | Reject | A blanket absence-normalization policy conflicts with nil guest facts that deliberately represent unknown state and trigger fail-closed decisions. |

## Discovery and ownership

- **Codex:** `AGENTS.md` is the native project instruction entrypoint. Canonical `.agents/skills/<name>/SKILL.md` files use repository skill discovery; `.agents` is not a general Codex configuration directory.
- **Claude Code:** `CLAUDE.md` imports shared standards and architecture. It explicitly routes to `.agents/skills` for task procedures. Native project skills are documented under `.claude/skills`; this repository deliberately omits that directory, so shared workflows are read as files rather than promised as `/skill-name` commands. See [Claude skill discovery](https://code.claude.com/docs/en/skills#choose-where-skills-load).
- **Claude reviewers:** `.claude/agents` contains three read-only reviewer definitions for Go, PowerShell and requested security audits. They reference shared guidance and use Claude's native [subagent format](https://code.claude.com/docs/en/sub-agents). They are not shared skill copies.
- **Other agents:** read the shared files through the root catalog or Copilot instructions. Markdown compatibility does not imply native skill discovery.

Shared standards live in `.agents/standards`, and the module map and run flow live in `.agents/architecture/overview.md`. `INVARIANTS.md` remains authoritative for hard constraints; Makefile owns executable checks. Both root instruction files reference the same knowledge. Tool-specific configuration belongs to its own host directory; no empty settings file, hooks or permission grants are added.

Edit reusable procedures only in `.agents/skills`. Maintain standard `name` and `description` frontmatter and update the root catalog when skills change. No `.claude/skills` adapters or duplicate bodies are maintained. Read selected workflow bodies explicitly when native discovery is unavailable; do not load the entire collection into every task. Skill activation grants no additional authority.

## Guidance and validation

Go guidance was checked against [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [context ownership](https://pkg.go.dev/context) and the [race detector's platform/toolchain requirements](https://go.dev/doc/articles/race_detector). PowerShell guidance follows Microsoft's [approved verbs](https://learn.microsoft.com/en-us/powershell/scripting/developer/cmdlet/approved-verbs-for-windows-powershell-commands) and [Windows PowerShell 5.1 help conventions](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_comment_based_help?view=powershell-5.1). This project's I19 documentation rules and 5.1 guest compatibility remain authoritative.

For skill changes, validate required frontmatter and matching folder names, resolve local Markdown links, check reviewer metadata and shared references, and inspect activation overlap. An available skill-creator validator can check the standard frontmatter; its success does not prove semantic quality. Do not use upstream's fixed-format checker against this differently structured collection.

Use concrete routing cases when reviewing the collection:

| Request | Relevant workflow and expected boundary |
| --- | --- |
| Change a guard deadline | Go development; use `run.now`, preserve recovery ownership, test with the simulated communicator, identify live guest coverage still needed. |
| Fix one-item Detect JSON | PowerShell development; inspect Go tags and test zero/one/multiple results under Windows PowerShell 5.1. |
| Refresh a Microsoft SCT package | Baseline update; verify provenance and regenerate data, route new lockout-sensitive settings to the protection workflow. |
| Correct an end-of-support date | Baseline update; verify the Microsoft Lifecycle date, without fabricating package or generated-data changes. |
| Review failing tests | Code review; keep real behavior under test and use existing seams without exporting private helpers or forcing a three-file batch. |
| Prepare a release or fix CI | Plugin release; use current workflow/toolchain evidence and distinguish unsigned snapshots from signed public assets. |
| Push a reviewed code commit | No automatic version tag, GitHub release or guest hardening operation. |

Repository validation commands remain in Makefile. In particular, `make check` includes `gofmt`, Go and PowerShell documentation checks, PSScriptAnalyzer, example formatting, staticcheck, build, vet, govulncheck, Go tests and Windows PowerShell Pester tests. `make generate-check` applies to configuration changes. If make is missing, run the recipe lines directly.

Structural validation and these routing reviews do not prove a model will select or obey a skill in every host. Report native discovery, model execution and live guest verification separately; none should be inferred from passing Go/Pester tests.

Historical validation of the original skill adoption (before the directory reorganization):

- The bundled skill-creator validator accepted all six canonical files and six Claude adapters. All 67 local Markdown links and both `CLAUDE.md` imports resolved; adapter metadata matched its canonical file.
- The adoption table accounts for all 20 upstream entrypoints at the pinned commit.
- Codex CLI 0.167.0 returned all six canonical project skills, enabled and without project skill errors, through its read-only `skills/list` API. No model turn was started.
- Claude Code 2.1.326 was present. Its documented project paths and adapter links were checked; slash-command execution and model-following behavior were not exercised.
- The `make check` recipe commands passed when run directly because make was unavailable: formatting, documentation checks, Go build/vet/staticcheck/tests, govulncheck, PowerShell analysis, example formatting and all 41 Windows PowerShell 5.1 Pester tests.
- Runtime code and build configuration were not changed by the skill adaptation. Live guest hardening, signed release publication and race testing are outside this documentation change.

## Directory reorganization

Shared standards and architecture are now extracted from the root instructions. The six canonical skills remain unchanged. The six former Claude discovery adapters were removed and replaced with three read-only Claude-specific reviewer definitions; these do not provide slash-command aliases. Historical adapter checks above do not describe the current layout. Validate current local links, Claude imports, reviewer frontmatter and canonical skill metadata when changing this structure. Native reviewer invocation and model-following behavior require separate host testing.

Current reorganization validation: 74 local Markdown links and seven Claude imports resolve; all six canonical skills and three read-only reviewers have valid matching metadata. The Makefile check recipes passed, including all 41 Windows PowerShell 5.1 Pester tests. Reviewer invocation and model behavior were not exercised.
