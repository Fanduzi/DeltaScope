# Application Config Lint Module

Derives deterministic rule-level replacement warnings for a DeltaScope YAML config file
without changing validation, audit, or policy behavior. This is the pure core behind the
future `deltascope config lint --file <path>` warning output; the CLI adapter and `--strict`
flag land in a later task.

## Files

| File | Responsibility |
|------|---------------|
| `lint.go` | `Inspect` entry point plus the `Request`/`Result`/`Warning` contract |
| `raw.go` | Raw YAML types and `parseRaw`, which distinguish omitted fields from zero values |
| `validate.go` | Semantic validation mirroring the CLI `config lint` and `configstatus` validators |
| `warn.go` | `deriveWarnings` emits the four v0.320.0 replacement-hazard cases in canonical order |
| `lint_test.go` | Verifies the four warning cases, error precedence, ordering, determinism, no-`severity`, and no-mutation |

## Exports

- `Request` — `{ Path }`
- `Result` — `{ Warnings []Warning }`
- `Warning` — `{ RuleID, Field, Message }`
- `Inspect(ctx, Request) (Result, error)`

## What it does

- Reads the YAML file, parses it with pointer-backed raw fields so omission is
  distinguishable from an explicit empty/zero value, and validates it with the same
  semantics as `deltascope config lint` (unknown rule, invalid level, unknown param, param
  type mismatch) plus malformed-YAML and missing/unreadable-file handling.
- Treats the Rule Catalog as the known-rule set, so default-disabled catalog-only rules such as `dml.impact.*` are valid opt-in config keys.
- Compares each mentioned rule against the shipped catalog/default policy snapshot and emits a warning
  when replacement changes effective behavior:
  1. default `enabled` is true but the override omits `enabled` (rule goes OFF),
  2. default `level` is non-empty but the override omits it (level becomes empty),
  3. default `params` exist but the override omits the whole `params` map, and
  4. default `params` exist and the override supplies a partial map (omitted default params
     are removed, reported one per key).
- Orders warnings by `rule_id` ascending, then by field: `enabled`, `level`,
  `params.<key>` alphabetical.
- Reports each warning with the `replaced, not partially merged` framing, the omitted field
  name, and the effective consequence.

## What it does not do

- It does not change validation semantics. Unknown rules, invalid levels, unknown params,
  param type mismatches, malformed YAML, missing files, and empty paths remain errors and
  take precedence over warnings.
- It does not load policy through `viperconfig.LoadPolicy`, run an audit, parse SQL, or
  connect to a database.
- It does not mutate `policy.Default()` or any parsed config map.
- It does not add a `severity` field; the public priority field is `level`.
- It does not render CLI text or define exit codes; that is the CLI adapter's job.

## Raw-config parser note

`raw.go` mirrors the on-disk YAML shape (`Enabled *bool`, `Level *string`,
`Params map[string]any`) already present in `internal/application/configstatus`. This task
keeps a local minimal copy here rather than refactoring the shipped `configstatus` and CLI
parsers, so it stays a focused, low-risk core. Consolidating the now-three raw-config
parsers (CLI `lintConfigFile`, `configstatus`, `configlint`) into a shared helper is a
deferred cleanup, tracked in
`docs/decisions/2026-06-15-v0.320.0-config-lint-warnings-docs-cleanup.md`.

## Dependencies

- Upstream: (none yet; the CLI `config lint` command will call `Inspect` in a later task)
- Downstream: `internal/domain/policy`, `internal/domain/rule`, `internal/domain/rule/catalog`

## Update Rule

- If members, interfaces, or dependencies change, update this file in the same change.
- The validation helpers mirror `internal/interfaces/cli` lint and `configstatus` validation
  on purpose; if those rules diverge, update all three together.
