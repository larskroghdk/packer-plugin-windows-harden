# ⚙️ Configuration reference

💡 All options are optional.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `baseline` | string | `auto` | `auto` picks the baseline for the detected OS. A baseline ID (see [README](../README.md#-supported-operating-systems)) pins it. The build fails if the pinned ID doesn't match the detected OS. |
| `profile` | string | `auto` | `domain_joined` or `non_domain_joined`. `auto` uses the detected domain membership. `non_domain_joined` applies Microsoft's `DeltaForNonDomainJoined` overrides. |
| `mode` | string | `compatibility` | `compatibility` defers settings that would break the Packer WinRM session and applies the rest. `strict` fails before making any change if anything would be deferred. |
| `dry_run` | bool | `false` | Detect, inspect and plan, then write the report without changing anything. Pending changes make the report `non_compliant`. |
| `exclude` | list(string) | `[]` | Setting ID patterns to skip. They match case-insensitively, `*` matches any sequence, and backslashes are literal. A pattern made only of `*` is rejected. |
| `fail_on_error` | bool | `true` | Fail the build when a setting fails to apply or verify. When `false`, failures are reported as warnings. |
| `reboot_if_required` | bool | `false` | Restart after security policy changes, wait for WinRM, then re-verify every compliant and hardened setting. |
| `restart_timeout` | duration | `30m` | How long to wait for WinRM after a restart, and after a guard rollback. Minimum `1m`. |
| `report_path` | string | `windows-harden-report-<build name>.json` | Where the [report](report.md) is written on the machine running Packer. Missing directories are created. |

## 🆔 Setting IDs

`exclude` patterns and report entries use these IDs:

| Kind | ID format | Example |
| --- | --- | --- |
| 🗝️ Registry | `registry:HKLM\<path>\<name>` | `registry:HKLM\Software\Policies\Microsoft\WindowsFirewall\DomainProfile\DefaultInboundAction` |
| 👤 User right | `user_right:<right>` | `user_right:SeDenyNetworkLogonRight` |
| 🔑 Account policy | `account_policy:<name>` | `account_policy:MinimumPasswordLength` |
| 📋 Audit policy | `audit_policy:<subcategory guid>` | `audit_policy:0cce923f-69ae-11d9-bed3-505054503030` |
| ⚙️ Service | `service:<name>` | `service:XblGameSave` |

⚠️ In HCL strings, backslashes must be escaped (`"registry:HKLM\\Software\\..."`).

## 📌 Requirements on the build

- The communicator must be `winrm`.
- The WinRM user must have an elevated administrator token. With a local account other than the built-in Administrator, this usually means `LocalAccountTokenFilterPolicy = 1`.
- The guest needs 64-bit Windows PowerShell 5.1.
