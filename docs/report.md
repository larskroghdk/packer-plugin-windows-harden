# 📊 Hardening report

Every run writes a JSON report to `report_path` and prints a summary, including runs that fail. `schema_version` is currently `1`, and it increments on incompatible changes.

## 📁 Top-level fields

| Field | Content |
| --- | --- |
| `schema_version`, `plugin` | Report format and plugin version |
| `started_at`, `finished_at` | UTC timestamps |
| `dry_run`, `mode` | Effective configuration |
| `system` | Detected OS, release, lifecycle status, edition, domain, roles, WinRM user, detection errors |
| `baseline` | Selected baseline ID, name, version, profile, source package (URL and SHA-256), enforced components, and `excluded` (components and settings outside the implemented scope, each with a reason) |
| `summary` | Counts per setting status |
| `compliance` | Overall result (see below) |
| `connectivity` | WinRM checks performed after changes and restarts (`name`, `at`, `passed`, `details`) |
| `reboot` | `required`, `performed`, `reason` |
| `warnings` | Non-fatal problems, such as domain Group Policy taking precedence |
| `settings` | One entry per evaluated setting (see below) |
| `deferred_plan` | Full definitions of the deferred settings, ready to apply in a later step once Packer no longer needs WinRM |

## 🚦 Setting status

| Status | Meaning |
| --- | --- |
| ✅ `compliant` | Already matched the baseline |
| 🛡️ `hardened` | Changed, then verified by re-inspection |
| ⏳ `pending` | Not compliant and not verified (dry run or an interrupted run) |
| ⏸️ `deferred` | Not applied, because it would break the Packer WinRM session. `reason` says why. |
| ⏭️ `skipped` | Matched an `exclude` pattern |
| ➖ `not_applicable` | The configured object doesn't exist, such as a service that isn't installed |
| ❌ `failed` | Inspection, apply or verification failed, or the setting no longer complied after the restart |

Each entry has `id`, `kind`, `description`, `component`, `status`, `expected`, and, where known, `before`, `after` and `reason`.

## 🏁 Compliance

| `compliance.status` | When |
| --- | --- |
| 🔴 `non_compliant` | Any setting is `failed` or `pending`, a connectivity check failed, or no baseline was evaluated |
| 🟡 `partial` | No failures, but some settings were `deferred` or `skipped` |
| 🟢 `compliant` | Every enforced setting is compliant |

`compliance.full_baseline` is `true` only when the status is `compliant` and the baseline has no excluded components or settings. `compliance.reasons` lists everything that kept the result from being fully compliant.

The optional top-level `error` field records a terminal provisioning failure, including failures after settings were verified. Such a report is always `non_compliant`.
