# internal/plan/

Compares inspected state with baseline settings, decides apply/guard/defer through the WinRM protection rules, and records verified outcomes. Parent: [AGENTS.md](../../AGENTS.md). Constraints: [INVARIANTS.md](../../INVARIANTS.md) (I6, I7, I9 apply directly).

## Local constraints

- The package is pure: no I/O and no guest calls. All input comes through `Context` and `State`, so every rule can be unit-tested in [plan_test.go](plan_test.go).
- A new rule in [protect.go](protect.go) needs a test that covers both outcomes (defer and apply), plus the unknown-state case.
- `Context.EmptyPassword` is the only credential signal. The password itself must never reach this package.

## Footguns

- State maps are keyed by lower-case setting ID. Look items up with `strings.ToLower(s.ID)`, or the lookup silently misses and the item is marked failed.
- Registry rule keys are lower-case `path\name` with no `HKLM\` prefix (`registryKey`), unlike setting IDs (`registry:HKLM\...`).
- The UAC settings (`EnableLUA`, `LocalAccountTokenFilterPolicy`, `FilterAdministratorToken`) interact. `uacModel` evaluates them in a fixed order and carries accepted values forward. Never decide them independently.
- `Compare` parses REG_DWORD values as uint32, so `4294967295` and `-1` in the guest output aren't interchangeable. The guest converts signed values to unsigned.
- A service that isn't installed returns `NotApplicable`, not `Mismatch`. Registry values never do.
