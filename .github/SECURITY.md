# Security policy

## Supported versions

Only the latest release receives security fixes. Fixes ship in a new release, not as backports.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |

## Reporting a vulnerability

Do not open a public issue, pull request or discussion.

Use [private vulnerability reporting](https://github.com/larskroghdk/packer-plugin-windows-harden/security/advisories/new) through the repository's "Report a vulnerability" button on the Security tab.

Include the plugin version, Packer version and builder, guest OS and build, the provisioner block with secrets removed, the relevant JSON report or `PACKER_LOG=1` output with secrets removed, steps to reproduce, and impact.

Do not include real credentials. Do not test against systems you do not own or are not authorized to test.

## Scope

In scope:

- Credentials, WinRM passwords or signing material exposed in reports, logs, uploaded files or release artifacts.
- Guest data reaching PowerShell command text instead of passing through the JSON contract.
- A setting reported as `hardened` that is not applied, or a guarded change whose rollback leaves the guest in a weaker or unknown state.
- A protection rule that applies a change when the guest state is unknown; protection rules must fail closed.
- Bypassing baseline provenance: a baseline not traceable to a pinned `download.microsoft.com` zip and SHA-256.
- Weaknesses in the release or CI workflows, including unpinned Actions or excessive token permissions, that could compromise published plugin binaries.

Out of scope:

- Microsoft's baseline content and vulnerabilities in Windows, WinRM or the Microsoft Security Compliance Toolkit. Report these to the [Microsoft Security Response Center](https://msrc.microsoft.com/report).
- Vulnerabilities in Packer or the Packer plugin SDK. Report these through [HashiCorp's security policy](https://www.hashicorp.com/security).
- Settings intentionally deferred to keep the Packer session working. These are documented in [WinRM protection](../docs/winrm-protection.md). Report an unexpected result as a plain bug.

## Process

The maintainer acknowledges reports, confirms or rejects the issue, and agrees a disclosure date with the reporter. The target acknowledgement time is within 7 days. This is a single-maintainer project, so timelines are best effort.

Fixes are released in a new version and disclosed through a GitHub Security Advisory. A CVE is requested through GitHub when appropriate. Reporters are credited unless they prefer not to be.

## Verifying releases

Download the SHA256SUMS file and its detached signature from the GitHub release. Verify the signature with GPG using the maintainer's release signing key, then check the plugin ZIP against the checksums.
