# 🔐 WinRM protection

Some baseline settings would cut off the session Packer uses to finish the build. Before applying anything, the plugin decides how to handle each non-compliant setting, based on the WinRM configuration and session identity it detected on the guest.

| Decision | Effect |
| --- | --- |
| ✅ apply | Applied with the main change set, then verified |
| ⏸️ defer | Not applied. Reported as `deferred` and listed in `deferred_plan`. |
| 🛡️ guard | Applied separately, with an automatic rollback (see below) |

🔒 When the information needed to decide is missing, the setting is deferred.

## 📏 Rules

| Setting | Deferred when |
| --- | --- |
| WinRM service `AllowBasic = 0` | Basic authentication is enabled on the service, or the WinRM configuration couldn't be read |
| WinRM service `AllowUnencryptedTraffic = 0` | Unencrypted traffic is allowed and Packer is, or may be, connected over an HTTP listener |
| `LimitBlankPasswordUse = 1` | Packer logs on as a local account with an empty password |
| `EnableLUA`, `LocalAccountTokenFilterPolicy`, `FilterAdministratorToken` | The resulting combination would give the Packer local account a filtered, non-elevated token on network logons (remote UAC). The three are evaluated together. |
| Firewall `EnableFirewall = 1`, `DefaultInboundAction = 1` | The profile carrying the WinRM connection is currently disabled or doesn't block inbound traffic by default, so the effect on the WinRM rule is unknown |
| Firewall `AllowLocalPolicyMerge = 0` | The profile carries the WinRM connection. Local rules, including the WinRM rule, would be ignored. |
| `SeDenyNetworkLogonRight` | It names a SID in the Packer session token |
| `SeNetworkLogonRight` | It doesn't grant any SID in the Packer session token |

🛡️ Two settings, `LmCompatibilityLevel` and `NtlmMinServerSec`, are **guarded** when Negotiate authentication is enabled. Whether the Packer client meets the new NTLM requirements can't be determined from the guest. Missing WinRM authentication facts defer these settings.

## 🛡️ Guarded apply

1. 📸 The current values are uploaded as a snapshot, and a one-time SYSTEM scheduled task is registered to restore them after 3 minutes.
2. ✍️ The settings are applied.
3. 📡 The plugin opens new WinRM commands for up to 90 seconds.
   - ✅ If WinRM works and nothing regressed, the task is removed and the settings are verified as usual.
   - ↩️ If WinRM works but something regressed, the settings are rolled back immediately.
   - ⏰ If WinRM can't be reached, the plugin waits for the scheduled rollback, then reconnects within `restart_timeout`.
4. 📝 Settings that were rolled back are reported as `deferred`, or as `failed` if the original value couldn't be confirmed.

The guest working directory permits access only to SYSTEM and administrators. Rollback and disarming atomically claim the snapshot so they cannot both proceed. If rollback is already running, disarming fails rather than reporting a successful guard check. The plugin preserves the scripts and snapshot whenever the guard may still be armed.

## 🔁 After every change set

A new WinRM command must run, and the WinRM service, listeners, authentication methods and unencrypted setting must be no weaker than before. Otherwise the build fails. The same check runs after a restart.

## 🗓️ Using deferred settings

Deferred settings are the hardening that only makes sense once Packer is gone. Apply `deferred_plan` from the report at first boot, or in the deployment pipeline, or set `mode = "strict"` to fail the build instead.
