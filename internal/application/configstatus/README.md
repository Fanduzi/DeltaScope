# Application Config Status Module

Derives the effective status of one shipped rule under the built-in default policy and an
optional YAML config file. This is the core data layer for the future
`deltascope config status <rule-id>` CLI command.

## Files

| File | Responsibility |
|------|---------------|
| `status.go` | `Inspect` derives ON/OFF, effective level, default vs current snapshots, and config effect for one rule |
| `status_test.go` | Verifies default-only, override, replacement-danger, validation, cloning, and JSON behavior |

## Exports

- `Request` — `{ RuleID, ConfigPath }`
- `Result` — `{ RuleID, Status, Default, Current, ConfigEffect, RuleDetailsCommand }`
- `RuleStatus` — `{ Enabled, Level, State }`
- `RulePolicySnapshot` — `{ Enabled, Level, Params }`
- `ConfigEffect` — `{ HasConfig, HasOverride, ChangedFields, Messages }`
- `Inspect(ctx, Request) (Result, error)`

## What it does

- Reads the built-in default policy (`policy.Default`) and the rule catalog
  (`catalog.Lookup`).
- Loads the effective policy via the existing `viperconfig.LoadPolicy` when a config path
  is supplied, and uses its output **verbatim** as the "current" snapshot.
- Compares default versus current `enabled`, `level`, and params, producing deterministic
  `ChangedFields` (`enabled`, `level`, then `params.<key>` alphabetically) and messages.
- Validates the config with the same semantics as `deltascope config lint` (unknown rule,
  invalid level, unknown param, param type mismatch) so malformed configs error instead of
  producing partial status.
- Clones all params maps before returning; it never mutates the default or effective policy.

## What it does not do

- It does not run an audit.
- It does not parse SQL.
- It does not connect to a database.
- It does not add a `severity` field; the public priority field is `level`.
- It does not modify `viperconfig.LoadPolicy` or audit behavior.

## Config semantics: rule-level replacement (audit-faithful)

The "current" snapshot matches what the audit path actually applies. Mentioning a rule in
YAML replaces its entire `RulePolicy`; fields the file omits become zero values
(`enabled` -> `false`, `level` -> `""`, `params` -> empty). Therefore a partial override
such as `level: warning` alone leaves the rule OFF, because `enabled` is omitted and
replaced with `false`. `Inspect` surfaces this with explicit omitted-field warnings rather
than hiding it. Whether `LoadPolicy` should adopt merge semantics is a separate decision
and is intentionally out of scope here. See
`docs/decisions/2026-06-14-v0.310.0-rule-config-status.md`.

## Dependencies

- Upstream: (none yet; the CLI `config status` command in Task 3 will call `Inspect`)
- Downstream: `internal/domain/policy`, `internal/domain/rule`,
  `internal/domain/rule/catalog`, `internal/infrastructure/config/viper`

## Update Rule

- If members, interfaces, or dependencies change, update this file in the same change.
- The semantic-validation helpers mirror `internal/interfaces/cli` lint behavior on
  purpose; if those rules diverge, update both together.
