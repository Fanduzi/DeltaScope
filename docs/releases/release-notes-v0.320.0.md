# DeltaScope v0.320.0 Release Notes

## Summary — Config Lint Replacement Warnings

v0.320.0 teaches `deltascope config lint` to warn when a rule entry in your YAML looks like it will silently disable or gut a rule. The warnings describe rule-level replacement semantics, which already governed how the audit applied policy; lint now surfaces the same footgun before an audit runs. By default lint prints the warnings and still exits 0. A new `--strict` flag prints the identical text but exits 2, for CI steps that want to fail on any warning.

This release does **not** change audit behavior, the config loader (`LoadPolicy`), the default policy, any rule, parser support, or any output shape. There is no `severity` field; DeltaScope uses `level`.

## The `config lint` Command

```bash
deltascope config lint --file <path> [--strict]
```

Examples:

```bash
deltascope config lint --file ./deltascope.yaml
deltascope config lint --file ./deltascope.yaml --strict
```

On a clean config the output is:

```text
Config OK
```

On a config with replacement hazards:

```text
Config OK with warnings

Warnings:
- rule "dml.where.require" is mentioned without "enabled"; the rule policy is replaced, not partially merged, so omitted "enabled" becomes false and the rule is OFF
- rule "dml.where.require" is mentioned without "params"; the rule policy is replaced, not partially merged, so omitted "params" become empty, removing the default params
```

### The Canonical Footgun

A user who wants to soften a rule level writes:

```yaml
rules:
  dml.where.require:
    level: warning
```

This looks like a one-field tweak. It is not. Mentioning a rule replaces that rule's whole policy, and omitted fields become their zero values:

| Field | Effective value when omitted |
|---|---|
| `enabled` | `false` |
| `level` | `""` (empty) |
| `params` | `nil` (empty) |

So the config above **turns the rule off**, because `enabled` is omitted and therefore replaced with `false`. lint now flags exactly this. To change only the level while keeping the rule on, write every field so the replacement leaves the others intact:

```yaml
rules:
  dml.where.require:
    enabled: true
    level: warning
    params:
      required: true
```

### `--strict` for CI

Default lint exits 0 even with warnings, because a warning is not an error and a config that mentions a rule without `enabled` can be intentional. `--strict` keeps the identical text output but returns exit code 2, so a CI step can fail the build on any replacement hazard:

```bash
deltascope config lint --file ./deltascope.yaml --strict
```

Errors remain errors and still exit non-zero without `--strict`. The warnings are text-only; there is no separate JSON or SARIF shape for them, and no `severity` field.

### Follow Up With `config status`

lint tells you a rule may be off; `config status` confirms it. For the footgun above:

```bash
deltascope --config ./deltascope.yaml config status dml.where.require
```

reports the rule is `OFF`, shows the effective `level`, and lists which fields your config changed from the default. See the v0.310.0 release notes for the full `config status` contract.

## Docs Cleanup

v0.320.0 also tidies public docs: the `config lint`, `config status`, and `rules explain` wording is clearer and consistent across English and Chinese. `docs/decisions/` is documented as maintainer rationale, not a user guide. These are documentation changes only; no behavior changes.

## Non-Goals

- Not audit behavior changes.
- Not config loader (`LoadPolicy`) changes; partial rule configs still do not merge onto defaults.
- Not default policy or rule changes.
- Not parser support changes.
- Not new audit rules.
- Not finding JSON shape changes.
- Not SDK/HTTP/MCP response shape changes.
- Not SARIF, GitHub Actions, or GitLab CodeQuality renderer changes.
- No `severity` field is introduced; `level` remains the public priority field.

## Rule Catalog Facts

The rule catalog is unchanged from v0.310.0. `config lint` warnings document existing semantics; they are not a rule change.

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
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).
- DDL coverage catalog: **400** entries (61 MySQL / 54 TiDB / 285 PostgreSQL / 18 parser_upgrade_candidate) (unchanged).
- Parser-error total: **29** cases across all dialects (unchanged).

## Decision Record

`docs/decisions/2026-06-15-v0.320.0-config-lint-warnings-docs-cleanup.md`
