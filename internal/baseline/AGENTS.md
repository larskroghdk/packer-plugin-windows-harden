# internal/baseline/ and cmd/sctimport/

Baseline model, the embedded catalog (`data/*.json` via `go:embed`), and the generator that produces it. Parent: [AGENTS.md](../../AGENTS.md). Constraints: [INVARIANTS.md](../../INVARIANTS.md) (I1–I5 apply directly). Procedure: [update-baseline](../../.agents/skills/update-baseline/SKILL.md).

## Local constraints

- A schema change (a new field, kind or registry type) has to land in three places together: the model in [baseline.go](baseline.go), `Validate`, and the generator. `Load` decodes with `DisallowUnknownFields`, so a generator-only change fails at plugin start.
- Supported registry types are `REG_DWORD`, `REG_SZ` and `ABSENT`. To add one, also extend `plan.Compare` and the guest `Set-SWRegistryValue` / `Get-SWRegistryValue`.
- Exclusions in `sources.json` need a `reason`. It ends up in the baseline's `excluded` list and in the report.

## Footguns

- The catalog is loaded and validated in `Prepare`, so one bad JSON file breaks every build, not just builds for that OS.
- Server 2025 and Windows 11 24H2 share build 26100. They are told apart by `product_type`, not build number, so overlap checks key on both.
- `NonDomainJoinedOverrides` replaces settings by ID, and an override with a new ID is appended. The generator rejects deltas that don't override an existing setting, so a new ID can only come from a hand edit.
- `sctimport` needs the SCT zips downloaded locally (`-downloads`). It never fetches them. The zip file name must match `sources.json` exactly.
- `TestEveryBaselineIsTraceable` fails if a baseline has fewer than 200 settings. A large drop after regeneration means parsing broke.
