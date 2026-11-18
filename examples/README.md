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

[local](local/) runs a locally built plugin against a Windows machine that already exists and has WinRM enabled, such as a Hyper-V VM. It uses the `null` builder, so it creates no image. It defaults to `dry_run = true`, which plans and writes the report without changing anything.

```sh
go build -o packer-plugin-windows-harden.exe .
packer plugins install --path packer-plugin-windows-harden.exe github.com/larskroghdk/windows-harden
cd examples/local
cp local.pkrvars.hcl.example local.pkrvars.hcl
packer build -var-file=local.pkrvars.hcl .
```

The template has no `required_plugins` block, so Packer uses the installed development build (`0.1.0-dev`). Set `host` and `password` in `local.pkrvars.hcl`, which git ignores. The report is written to `reports/local.json`.

⚠️ With `dry_run = false` the plugin hardens the target machine and restarts it if needed. Use a disposable VM or take a checkpoint first. The defaults connect with NTLM over HTTP on port 5985. Set `use_ssl = true` for HTTPS.
