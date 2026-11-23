# 🧪 Examples

| Example | Builder | Shows |
| --- | --- | --- |
| ☁️ [azure-arm](azure-arm/) | `azure-arm` | Marketplace image, hardening, then sysprep to a managed image |
| 🖥️ [vsphere-iso](vsphere-iso/) | `vsphere-iso` | Unattended install from ISO, then hardening, converted to a template |
| 🏠 [local](local/) | `null` | A locally built plugin run against an existing Windows VM, for testing |

⚙️ Options are described in [docs/configuration.md](../docs/configuration.md).

## 🧩 One template per builder

`azure-arm` and `vsphere-iso` each build every supported OS from one template. A variable file per baseline pins the baseline ID and selects the source image:

| Variable file | OS | `azure-arm` image | `vsphere-iso` guest OS type |
| --- | --- | --- | --- |
| `ws2022.pkrvars.hcl` | Windows Server 2022 Datacenter | `2022-datacenter-azure-edition` | `windows2019srvNext_64Guest` |
| `ws2025-2602.pkrvars.hcl` | Windows Server 2025 Datacenter | `2025-datacenter-azure-edition` | `windows2022srvNext_64Guest` |
| `win11-24h2.pkrvars.hcl` | Windows 11 Enterprise 24H2 | `win11-24h2-ent` | `windows11_64Guest` |
| `win11-25h2.pkrvars.hcl` | Windows 11 Enterprise 25H2 | `win11-25h2-ent` | `windows11_64Guest` |
| `win11-26h2.pkrvars.hcl` | Windows 11 Enterprise 26H2 | `win11-26h2-ent` | `windows11_64Guest` |

```sh
cd examples/vsphere-iso
packer init .
packer build -var-file=ws2025-2602.pkrvars.hcl -var-file=site.pkrvars.hcl .
```

🏠 `site.pkrvars.hcl` is yours. It holds the values the per-OS file doesn't set: `subscription_id` and `resource_group` for `azure-arm`, and the vCenter connection, placement, network and `admin_password` for `vsphere-iso`. `profile`, `mode` and `exclude` can be set there too.

⚠️ For `vsphere-iso`, edit `os_iso_path` in the per-OS file to match your ISO datastore. The unattended install turns on WinRM over HTTP with Basic authentication and unencrypted traffic so Packer can connect. The baseline disables both, but the plugin defers those settings so the session survives, which means the template keeps them enabled (see [docs/winrm-protection.md](../docs/winrm-protection.md)). Apply `deferred_plan` from the report at first boot. With `mode = "strict"`, this example fails before making any change.

## 🏠 Local test

[local](local/) runs a locally built plugin against a Windows machine that already exists and has WinRM enabled, such as a Hyper-V VM. It uses the `null` builder, so it creates no image. It defaults to `dry_run = true`, which plans and writes the report without applying baseline settings.

The computer running [Test-Local.ps1](local/Test-Local.ps1) needs PowerShell 5.1 or later, the Go version specified in `go.mod` or newer, and Packer on PATH. The target must run a [supported OS](../README.md#-supported-operating-systems), allow WinRM connections from that computer and provide an elevated administrator session in 64-bit Windows PowerShell. The script does not create a VM or configure WinRM. Run the following setup commands from the repository root:

```powershell
Copy-Item examples/local/local.pkrvars.hcl.example examples/local/local.pkrvars.hcl
notepad examples/local/local.pkrvars.hcl
./examples/local/Test-Local.ps1
```

Run the script from PowerShell with Go and Packer on PATH. Set `host`, `username` and `password` in `local.pkrvars.hcl`, which git ignores. The script builds and installs the plugin at the version in `main.go` (for example `0.1.0`) into an isolated `dist/local-test/plugins` directory and validates the template before running it. It restores the caller's plugin path and working directory afterward. The report is written to `examples/local/reports/local.json`; failed settings cause the script to fail.

Use `-ValidateOnly` to check the local build and template without connecting to a target. Use `-VarFile <path>` for another `.hcl` or `.json` variable file. Relative variable-file paths are resolved from your current directory.

| Command from the repository root | Checks |
| --- | --- |
| `./examples/local/Test-Local.ps1 -ValidateOnly` | Builds and installs the plugin and validates HCL; no target connection or report. The copied example values can be used unchanged. |
| `./examples/local/Test-Local.ps1` | Connects to the configured target, detects state, plans changes and writes the report without applying baseline settings. |
| `./examples/local/Test-Local.ps1 -Apply` | Applies hardening, verifies results and restarts the target if required. |
| `./examples/local/Test-Local.ps1 -VarFile C:/test/target.pkrvars.hcl` | Runs a dry-run inspection using another target configuration. |

The script forces a dry run even when the variable file sets `dry_run = false`. Pass `-Apply` to harden the target machine and restart it if needed. Use a disposable VM or take a checkpoint first. A successful dry run verifies inspection and planning; run with `-Apply` to test application and post-apply verification. The defaults connect with NTLM over HTTP on port 5985. Set `use_ssl = true` for HTTPS with a trusted guest certificate. The vSphere example also requires a trusted vCenter certificate. Azure's temporary build VM uses a self-signed WinRM certificate, so its example retains the builder's certificate verification exception for that connection.

Inspect `summary`, `compliance`, `connectivity` and `deferred_plan` in the [JSON report](../docs/report.md). A successful script run does not mean every setting is compliant: a dry run can contain pending changes, and compatibility mode can defer settings to preserve WinRM. The script fails if a command fails, the expected report is missing or the report contains failed settings. Reports may also exist after a failed Packer run; check their timestamps before using them.

If the variable file is missing, copy the example as shown above; Packer cannot load the `.hcl.example` filename directly. For connection or elevation errors, check the target's WinRM listener, firewall and account permissions against the [build requirements](../docs/configuration.md#-requirements-on-the-build). `-ValidateOnly` cannot verify any of those guest requirements.
