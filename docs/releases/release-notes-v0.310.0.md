# DeltaScope v0.310.0 Release Notes

## Summary — Rule Config Status

v0.310.0 adds a focused CLI inspection command, `deltascope config status <rule-id>`, that reports whether one shipped rule is ON or OFF under the active config and which `level` it will use if it fires. It also explains how the user's config changed the rule versus the built-in default policy, and points at `deltascope rules explain <rule-id>` for rule meaning. This release does **not** add new audit rules, change rule behavior, change audit behavior, change finding JSON shape, add parser support, or change SDK/HTTP/MCP outputs. There is no `severity` field; DeltaScope uses `level`.

## The `config status` Command

```bash
deltascope config status <rule-id> [--format text|json]
```

Examples:

```bash
deltascope config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require
deltascope --config ./deltascope.yaml config status dml.where.require --format json
```

The command answers one practical question: *given my current config file, is this rule on or off, and what level will it use if it fires?*

It reports:

- Whether the rule is currently **ON** or **OFF**.
- The effective `level` the rule uses when on (`blocker`, `warning`, or `notice`).
- Whether the user's config changed `enabled`, `level`, or params from the default policy, with the default and current values shown side by side.
- A direct next step: `deltascope rules explain <rule-id>` for rule meaning.

### Config Source

The command uses the existing global `--config` flag, matching `deltascope audit`. When `--config` is omitted, `config status` reports the built-in default policy and states that no config override is active. `config lint --file` remains the validation-only command and is unchanged.

### Output Formats

- `--format text` (default) is intended for humans: it states `ON` or `OFF`, the effective `level`, a concise config-effect explanation, the default and current values, and a `Rule details:` link to `deltascope rules explain <rule-id>`.
- `--format json` returns a stable wrapper for automation. JSON output includes a top-level `version` field (the DeltaScope build version), `rule_id`, `status` (`enabled`, `level`, `state`), `default`, `current`, `config_effect`, and `rule_details_command`. There is no `severity` field in the JSON output.

Minimal JSON shape (full-spec override, only `level` differs):

```json
{
  "version": "v0.310.0",
  "rule_id": "dml.where.require",
  "status": { "enabled": true, "level": "warning", "state": "on" },
  "config_effect": { "has_config": true, "has_override": true, "changed_fields": ["level"] },
  "rule_details_command": "deltascope rules explain dml.where.require"
}
```

## Rule-Level Replacement Semantics

`config status` reports the effective policy **exactly as the audit path applies it**. It reads the loaded policy verbatim and never silently merges a partial rule config onto the defaults, because audit does not do that either.

Mentioning a rule in the YAML replaces that rule's whole policy: omitted fields become their zero values. This is not a partial merge.

| Field | Effective value when omitted |
|---|---|
| `enabled` | `false` |
| `level` | `""` (empty) |
| `params` | `nil` (empty) |

**Consequence:** a user who writes only

```yaml
rules:
  dml.where.require:
    level: warning
```

expecting to soften the level actually **turns the rule off**, because `enabled` is omitted and therefore replaced with `false`. `config status` surfaces this explicitly rather than hiding it. To change only the level while keeping the rule on, specify every field so the replacement leaves the others intact:

```yaml
rules:
  dml.where.require:
    enabled: true
    level: warning
    params:
      required: true
```

Whether the loader should adopt merge semantics instead is a separate, larger decision and is out of scope for v0.310.0. v0.310.0 does not change the loader, audit behavior, or default policy values.

## Non-Goals

- Not new audit rules.
- Not rule behavior changes.
- Not audit behavior changes.
- Not finding JSON shape changes.
- No `severity` field is introduced; `level` remains the public priority field.
- Not parser support changes.
- Not SDK/HTTP/MCP config-status surfaces.
- Not a bulk `config effective` command (single-rule status only).
- The command does not run an audit.
- The command does not parse SQL.
- The command does not connect to a database.

## Rule Catalog Facts

The rule catalog is unchanged from v0.300.0. `config status` is an inspection command, not a rule change.

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

`docs/decisions/2026-06-14-v0.310.0-rule-config-status.md`
