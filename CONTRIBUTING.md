# 🤝 Contributing

🐛 Bug reports, baseline updates and protection rules are welcome. Open an issue before starting larger changes so we can agree on the approach.

Participation is governed by the [Code of Conduct](.github/CODE_OF_CONDUCT.md). Report vulnerabilities privately by following [SECURITY.md](.github/SECURITY.md), not in issues.

## 🧰 Setup

You need the Go patch version specified in `go.mod` or newer, `staticcheck`, `govulncheck`, Packer, `pwsh` with PSScriptAnalyzer and Pester 5, and Windows PowerShell 5.1 for the Pester run. `make init` installs the check and configuration-generation tools. `make check` includes a vulnerability scan that requires access to the Go vulnerability database.

CI snapshots and releases use the official prebuilt GoReleaser action. Local packaging targets install GoReleaser when needed; its install recipe uses `GOTOOLCHAIN=auto` because the tool requires a newer compiler than the plugin. The override applies only to tool installation, while plugin builds and checks use the project toolchain.

⚠️ `make` isn't on the default Git Bash PATH on Windows. If it's missing, run the recipe lines from the [Makefile](Makefile) directly.

## ✅ Before you open a pull request

- 🧪 Run `make check` (lint, typecheck and tests). It must pass.
- 📜 Read [INVARIANTS.md](INVARIANTS.md). A change that breaks an invariant won't be merged, even if the tests pass.
- 🐞 Add a test for every bug you fix. Guest-script tests must pass under Windows PowerShell 5.1, not only pwsh.
- 🏗️ `make check` doesn't cover orchestration or a real WinRM build. For changes to `provisioner/` or the guest scripts, run a Packer build against a supported guest and attach the report to the pull request.

## 📏 Code rules

- 📝 **No comments except documentation (I19).** Every Go function and method gets a doc comment in this format, checked by `cmd/doccheck`:

  ```go
  // Select returns the baseline for a product type and build number.
  //
  // Returns:
  //   - *Baseline: The matching baseline.
  //   - error: No baseline is implemented for the combination.
  //
  // Author: Lars Krogh
  // Version: 1.0.0
  ```

  Every PowerShell script and function gets comment-based help with `.SYNOPSIS`, `.DESCRIPTION`, `.PARAMETER`, `.EXAMPLE`, `.OUTPUTS` and `.NOTES` (`Author`, `Version`). Keep the existing author and raise the version when behavior changes.
- 🧠 **Decisions belong in Go (I15).** Guest scripts only detect, read and write state.
- ⚙️ Run `make generate` after changing a `mapstructure` field in `provisioner/config.go`.
- 🔢 Increment `report.SchemaVersion` for an incompatible change to the report JSON.

## 📚 Baselines

🚫 Never edit `internal/baseline/data/*.json` by hand. Add or update the package in `cmd/sctimport/sources.json` with its `download.microsoft.com` URL and SHA-256, then run `make baselines DOWNLOADS=<dir with SCT zips>`. See [.agents/skills/update-baseline/SKILL.md](.agents/skills/update-baseline/SKILL.md).

## Agent skills

The [skill collection](docs/agent-guidance.md) covers development, review, releases and baseline safety. Edit shared workflows only in `.agents/skills`; there are no per-tool copies to keep in sync. Validate metadata, local links and trigger scope when changing a skill. Shared coding rules remain in `INVARIANTS.md` and module guides rather than copied into every skill.

The maintainer's agent workflow, with its Codex review before each commit, is described in [docs/ai-development.md](docs/ai-development.md). Its Git hooks are opt-in: nothing runs until you install them, and CI runs the same checks on every pull request either way.

## 🔐 WinRM protection rules

A setting that could break the Packer session needs a rule in `internal/plan/protect.go`, and that rule must fail closed when guest state is unknown. See [.agents/skills/add-protection-rule/SKILL.md](.agents/skills/add-protection-rule/SKILL.md) and [docs/winrm-protection.md](docs/winrm-protection.md).
