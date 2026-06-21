# DeltaScope v0.360.0 Release Notes

## Summary — Config + Rule Explain UX

v0.360.0 sharpens the human-readable handoff between the three commands that explain how a YAML config affects one rule: `config lint`, `rules explain`, and `config status`. No primitive, behavior, or machine-readable contract changes — only the copy that connects them.

`config lint` replacement-hazard warnings are now direct and task-oriented. Each warning states plainly that the rule policy is replaced rather than merged with defaults, spells out the omitted-field consequence, and ends with an `Inspect effective rule status:` line pointing at `deltascope config status <rule-id> --config <path>`.

`rules explain <rule-id>` text output gains three blocks: a `Default policy:` block built from the actual default policy, a `Safe override example:` block that keeps the default `enabled` and `params` while changing `level`, and an `Inspect effective rule status:` handoff pointing at `config status`.

The public docs now describe the `config lint → rules explain → config status` workflow in English and Chinese, matching the new text contracts.

This is guidance text only. It does not change audit behavior, the default policy, any rule, parser support, or any machine-readable output shape. There is no `severity` field; DeltaScope uses `level`.

## What Changed in the CLI Text

- `config lint` warning copy now reads as a multi-line block. For each replacement hazard it names the consequence directly:
  - omitted `enabled`: `<rule> is OFF because "enabled" is omitted.`
  - omitted `level`: `<rule> has no effective level because "level" is omitted.`
  - omitted whole `params`: `<rule> removes default params because "params" is omitted.`
  - omitted `params.<key>`: `<rule> removes default "params.<key>" because that key is omitted.`
  - framing: `This config replaces the whole rule policy; it does not merge with defaults.`
  - handoff: `Inspect effective rule status:` followed by `deltascope config status <rule-id> --config <file path>`.
- `rules explain` text now renders `Default policy:`, `Safe override example:`, and `Inspect effective rule status:`. The previous `Config Example:` block was generated from `policy.Default()` and was byte-identical to the new `Default policy:` block, so it was removed to avoid printing the default policy twice. `Default Params:` is retained as the compact params-only view.
- The handoff label is `Inspect effective rule status:`. `Next:` is not used, because it describes the concrete inspection action rather than implying a generic wizard flow.

## What Stayed the Same

- JSON output for `config lint`, `config status`, and `rules explain` is unchanged. `rules explain --format json` still carries the catalog-generated `config_example` field.
- Exit codes are unchanged. `config lint --strict` produces the same stdout as the default mode for warning-only results; it only changes the exit code to `2`. Validation errors still take precedence and suppress the warning block.
- `config status` remains the single-rule ON/OFF check.
- Warning ordering stays deterministic.

## Non-Goals

- Not `LoadPolicy` behavior changes.
- Not partial-merge semantics for rule policy.
- Not default policy changes.
- Not audit behavior changes.
- Not parser support changes.
- Not new audit rules.
- Not finding JSON shape changes.
- Not SDK/HTTP/MCP response shape changes.
- Not JSON, SARIF, GitHub Actions, or GitLab Code Quality renderer behavior changes.
- Not a config auto-fix command.
- Not a full effective-policy dump.
- Not CLI Chinese / i18n output. Chinese guidance lives in docs only.
- No `severity` field is introduced; `level` remains the public priority field.

## Rule Catalog Facts

The rule catalog is unchanged from v0.340.0. This release moves text only; it is not a rule change.

| Metric | Count |
|--------|------:|
| Total rules | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE config entries: **53** (unchanged).
- DDL coverage catalog: **400** entries (unchanged).
- Parser-error total: 29 cases across all dialects (unchanged).

## Decision Record

`docs/decisions/2026-06-20-v0.360.0-config-rule-explain-ux.md`
