---
name: update-baseline
description: Add a new Microsoft security baseline or refresh an existing one (new SCT package, new Windows release or build). Use when sources.json, internal/baseline/data/*.json or the OS lifecycle table must change.
---

# Update a baseline

Read [INVARIANTS.md](../../../INVARIANTS.md) I1–I5 and I13, and [internal/baseline/AGENTS.md](../../../internal/baseline/AGENTS.md). Commands and code paths below are relative to the repository root.

For a lifecycle-only correction, use steps 1, 4, 5 and 7; package specification, generation and package-diff steps apply when the SCT package changes. Update selection tests only if applicability changes.

## Steps

1. **Establish the source.** For a package change, verify the Microsoft SCT download URL and package version against Microsoft's current documentation. Download the zip into an allowed scratch directory, such as ignored `dist/sct-downloads`, and record its SHA-256 (`Get-FileHash -Algorithm SHA256` on Windows). Inspect package contents without running vendor installation scripts. For a lifecycle-only correction, verify the Microsoft Lifecycle source and leave unchanged package metadata and generated data alone.
2. **Edit `cmd/sctimport/sources.json`.** For a refresh, update the existing entry; for a new release, add one. Fill in:
   - `id` (file name of the output, e.g. `ws2025-2602`), `name`, `version`, `zip` (exact file name), `url`, `sha256`
   - `policy_rules`, `workbook`, `delta_inf`, `delta_txt`: paths inside the zip
   - `product_type` (`server` or `workstation`) and `builds`. No other baseline may claim the same pair.
   - `components` to enforce, and `excluded_components` with a `reason` for **every** other GPO in the package
   - `excluded_settings` with reasons, only for individual settings the plugin deliberately doesn't enforce
3. **Generate:** `make baselines DOWNLOADS=<scratch dir>`. Fix every error by changing `sources.json`, or by extending the generator if the package has a construct it doesn't support yet. Never by editing the output.
4. **Update lifecycle data only when needed** in `internal/facts/lifecycle.go` (`serverReleases`, `clientReleases` and `ltscReleases`) using verified Microsoft Lifecycle dates. For clients, distinguish consumer and enterprise dates and editions. Client LTSC editions follow the fixed lifecycle of their LTSC release, with separate Enterprise and IoT Enterprise dates, and use the baseline of the general availability release Microsoft lists as equivalent. Classifying a release as supported does not establish that a baseline exists for it.
5. **Update the tests:**
   - `TestCatalogSelect` (internal/baseline/catalog_test.go): add the new `(product_type, build) → id`. If a build moved to a new baseline ID, change its expected ID.
   - `TestClassify` (internal/facts/facts_test.go): add the new release.
6. **Review the generated JSON diff.** For a refresh, check that the setting count and removed or changed settings match Microsoft's change log. Check that every new protection-sensitive setting is covered by an existing rule in `internal/plan/protect.go`: WinRM, UAC, NTLM, firewall, network-logon rights, or password expiry. If one isn't, follow [add-protection-rule](../add-protection-rule/SKILL.md). If extending a setting kind or registry type, update model validation, generator, comparison and guest reads/writes together.
7. **Run `make check`**, then update the supported-OS table in `README.md`.

## Done when

- `make check` passes.
- Changed source metadata, generated JSON, applicable lifecycle data, tests and the supported-OS documentation agree. Do not create unrelated edits merely to make every file change.
- For a lifecycle-only correction, run the classification tests and `make check`; generation and catalog-selection test changes are needed only when applicability or package content changes.
