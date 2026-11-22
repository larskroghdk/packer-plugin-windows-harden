---
name: release-plugin
description: Prepare, debug or verify this plugin's GitHub Actions builds, Go dependency updates, GoReleaser artifacts and tagged GitHub releases. Use for CI failures, release readiness or publication; skill selection alone does not authorize a tag or release.
---

# Plugin builds and releases

Read the current [build workflow](../../../.github/workflows/build.yml), [release workflow](../../../.github/workflows/release.yml), [GoReleaser config](../../../.goreleaser.yml), [Makefile](../../../Makefile) and [go.mod](../../../go.mod). Resolve commands and paths from the repository root; inspect current behavior rather than assuming earlier release instructions still apply.

## Build or dependency work

- Identify the failed step, commit, runner and shell from its logs. Windows checks use UCRT64, host `pwsh` analysis and Windows PowerShell 5.1 tests; signed publication runs on Linux after the reusable Windows checks.
- Use the declared Go toolchain and pinned tools. When Go changes, rebuild tools that reject a newer project toolchain. Let `go get` and `go mod tidy` maintain module checksums; review the resulting diff and run `make vulncheck` after security updates.
- Keep ordinary CI snapshots independent of signing secrets. `make release-snapshot` skips publication and signing; it is not evidence that a signed GitHub release succeeded.
- Preserve immutable action pins and minimum job permissions. Dependency updates do not justify disabling checksum verification, vulnerability checks, or guest-runtime coverage.

## Prepare a release

1. Resolve the intended version, target repository and candidate commit from the user's request and Git history. Preserve unrelated changes. A successful branch push is not itself a request to publish a version.
2. Run `make check` and `make generate-check`; inspect the candidate's changes and release notes. Build a snapshot when packaging or cross-compilation changed. Keep generated artifacts out of commits.
3. Check local and remote tag names, existing releases and assets. Confirm the candidate contains the release workflow. A release page or an existing tag without uploaded assets is not a completed release. Lookup or authentication errors are not proof of absence.
4. Confirm `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` are configured through the repository's Actions secrets, without printing their values. Obtain `API_VERSION` from the built plugin's `describe` result; do not hard-code the SDK API suffix.
5. Establish publication authorization for the selected version and commit. Prepare a concrete candidate before requesting missing approval, and honor approval already given in the current task. A tag push publishes a public release because `release.draft` is false.

## Publish and verify when authorized

Create the version tag at the selected commit and push that tag explicitly, rather than pushing all local tags. Watch the release workflow for that tag and commit. Inspect partial publication before retrying; do not move an existing release tag or replace published assets as an automatic recovery step.

Check the six ZIPs configured for Linux, macOS and Windows on amd64/arm64, the SHA-256 manifest and its detached GPG signature. Compare archive and binary names with the detected plugin API version. Verify checksums and the signature against the intended signing key, and run `describe` on an extracted binary for the local host. Distinguish a GitHub workflow artifact from a GitHub Release asset.

Report the tag, commit, run URL, observed assets and verification results. If publication or verification is incomplete, identify the failed checkpoint without claiming release success. This repository publishes a Packer plugin; there is no application deployment, container registry or PowerShell Gallery publication step.
