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

Requires Go 1.24+, `staticcheck`, `pwsh` with PSScriptAnalyzer and Pester 5, and `packer-sdc` for `make generate`.

```sh
make check      # lint + typecheck + test
make build      # build ./packer-plugin-windows-harden
make generate   # regenerate provisioner/config.hcl2spec.go
make baselines DOWNLOADS=<dir>   # regenerate baseline data from SCT zips
```

📦 Releases are published by pushing a `v*` tag (see [.github/workflows/release.yml](.github/workflows/release.yml)).

🤖 Contributor and agent guidance: [AGENTS.md](AGENTS.md) and [INVARIANTS.md](INVARIANTS.md). Task procedures: [update-baseline](.agents/skills/update-baseline/SKILL.md) and [add-protection-rule](.agents/skills/add-protection-rule/SKILL.md).
