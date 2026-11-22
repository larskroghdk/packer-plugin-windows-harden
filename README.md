# 🔒 packer-plugin-windows-harden

A Packer provisioner that applies the Microsoft Security Compliance Toolkit baseline that matches the guest OS. It runs over WinRM and doesn't apply changes that would cut off Packer's own session.

## ✨ What it does

1. 🔍 Detects the OS, edition, domain membership, the WinRM configuration and the Packer session identity.
2. 🎯 Selects the matching baseline and profile (`domain_joined` or `non_domain_joined`).
3. 🧭 Inspects every setting and plans the changes. A setting that would break the Packer WinRM session is ⏸️ **deferred**. A setting whose effect can't be determined from the guest, such as NTLM levels, is applied under an automatic rollback 🛡️ **guard**. See [WinRM protection](docs/winrm-protection.md).
4. ✅ Applies the changes, verifies each one by re-inspection, checks WinRM, and can restart and re-verify.
5. 📊 Writes a JSON [report](docs/report.md).

## 💻 Supported operating systems

| Baseline ID | OS | Build |
| --- | --- | --- |
| `ws2019-member` | Windows Server 2019 (member server) | 17763 |
| `ws2022` | Windows Server 2022 (member server) | 20348 |
| `ws2025-2602` | Windows Server 2025 (member server) | 26100 |
| `win11-24h2` | Windows 11 24H2 | 26100 |
| `win11-25h2` | Windows 11 25H2 | 26200 |
| `win11-26h2` | Windows 11 26H2 | 26300 |

🚫 Domain controllers, LTSC editions and releases past end of support are rejected.

## 🚀 Usage

```hcl
packer {
  required_plugins {
    windows-harden = {
      source  = "github.com/larskroghdk/windows-harden"
      version = ">= 0.1.0"
    }
  }
}

build {
  sources = ["source.hyperv-iso.ws2025"]

  provisioner "windows-harden" {
    reboot_if_required = true
  }
}
```

🔌 The communicator must be `winrm`, with an elevated administrator session. All options are described in [docs/configuration.md](docs/configuration.md). Complete templates are in [examples/](examples/).

## 🔧 Development

Requires the Go version in `go.mod` or newer, `staticcheck`, `govulncheck`, Packer, `pwsh` with PSScriptAnalyzer and Pester 5, and Windows PowerShell 5.1 for guest tests. `make init` installs the Go tools, including `packer-sdc` for `make generate`. The vulnerability scan in `make check` needs access to the Go vulnerability database.

```sh
make check      # lint + typecheck + test
make build      # build a snapshot binary under dist/
make generate   # regenerate provisioner/config.hcl2spec.go
make baselines DOWNLOADS=<dir>   # regenerate baseline data from SCT zips
```

For a local smoke test, use [examples/local/Test-Local.ps1](examples/local/Test-Local.ps1). It builds an isolated development plugin and runs the local template against an existing Windows WinRM target. See [local test setup and commands](examples/README.md#-local-test) for credentials, validation without a target, dry-run inspection and applying hardening.

📦 The [build workflow](.github/workflows/build.yml) checks generated configuration, runs `make check`, and uploads unsigned snapshot artifacts to the workflow run. Pushing a version tag such as `v0.1.0` starts the [release workflow](.github/workflows/release.yml), which builds plugin ZIPs for Linux, macOS, and Windows (amd64 and arm64) and publishes them to GitHub Releases with SHA-256 checksums and a GPG signature. Configure the repository Actions secrets `GPG_PRIVATE_KEY` (an ASCII-armored private signing key) and `GPG_PASSPHRASE` (its passphrase) before releasing. PowerShell Gallery is used only to install lint and test dependencies; nothing is published there.

🤖 Contributor and agent guidance: [AGENTS.md](AGENTS.md) and [INVARIANTS.md](INVARIANTS.md). Task procedures: [update-baseline](.agents/skills/update-baseline/SKILL.md) and [add-protection-rule](.agents/skills/add-protection-rule/SKILL.md).
