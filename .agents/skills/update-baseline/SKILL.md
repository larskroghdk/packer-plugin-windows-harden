---
name: update-baseline
description: Add a new Microsoft security baseline or refresh an existing one (new SCT package, new Windows release or build). Use when sources.json, internal/baseline/data/*.json or the OS lifecycle table must change.
---

# Update a baseline

Constraints: INVARIANTS.md I1–I5 and I12. Module context: `internal/baseline/AGENTS.md`.

## Steps

1. **Get the package.** Download the Security Compliance Toolkit zip from `https://download.microsoft.com/...` into a scratch directory outside the repo, and record its SHA-256 (`sha256sum <zip>`).
2. **Edit `cmd/sctimport/sources.json`.** For a refresh, update the existing entry; for a new release, add one. Fill in:
   - `id` (file name of the output, e.g. `ws2025-2602`), `name`, `version`, `zip` (exact file name), `url`, `sha256`
   - `policy_rules`, `workbook`, `delta_inf`, `delta_txt`: paths inside the zip
   - `product_type` (`server` or `workstation`) and `builds`. No other baseline may claim the same pair.
   - `components` to enforce, and `excluded_components` with a `reason` for **every** other GPO in the package
   - `excluded_settings` with reasons, only for individual settings the plugin deliberately doesn't enforce
3. **Generate:** `make baselines DOWNLOADS=<scratch dir>`. Fix every error by changing `sources.json`, or by extending the generator if the package has a construct it doesn't support yet. Never by editing the output.
4. **Update the lifecycle table** in `internal/facts/lifecycle.go` (`serverReleases` / `clientReleases`) with the Microsoft Lifecycle end-of-support dates for any new build. For clients, include both the consumer and enterprise dates.
5. **Update the tests:**
   - `TestCatalogSelect` (internal/baseline/catalog_test.go): add the new `(product_type, build) → id`. If a build moved to a new baseline ID, change its expected ID.
   - `TestClassify` (internal/facts/facts_test.go): add the new release.
6. **Review the diff of `internal/baseline/data/<id>.json`.** For a refresh, check that the setting count and the removed or changed settings match Microsoft's change log for the package. Check that every new protection-sensitive setting is covered by an existing rule in `internal/plan/protect.go`: WinRM, UAC, NTLM, firewall, or network-logon rights. If one isn't, follow the add-protection-rule skill.
7. **Run `make check`**, then update the supported-OS table in `README.md`.

## Done when

- `make check` passes.
- `sources.json`, the generated JSON, `lifecycle.go`, the tests and `README.md` all change in one commit.
