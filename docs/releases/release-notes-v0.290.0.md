# DeltaScope v0.290.0 Release Notes

## Summary — Rule Discoverability and Explain

v0.290.0 adds CLI commands for browsing and explaining DeltaScope's built-in rule catalog. `deltascope rules list` lists rules with filters for dialect, level, kind, category, and free-text search. `deltascope rules explain <rule_id>` shows full metadata for one rule. Both commands support JSON output for automation. This release does **not** add new audit rules, change rule evaluation behavior, change finding JSON shape, add a `severity` field, rename `level` to `severity`, change parser support, or add SDK/HTTP/MCP rule discovery surfaces.

## CLI Rule Discovery

Two new commands let users explore the rule catalog without running audits:

```bash
# List all rules
deltascope rules list

# Filter by dialect
deltascope rules list --dialect postgresql

# Filter by level
deltascope rules list --level blocker

# Filter by kind
deltascope rules list --kind dml

# Free-text search
deltascope rules list --search "primary key"

# JSON output
deltascope rules list --format json

# Explain one rule in detail
deltascope rules explain dml.where.require

# Explain with JSON output
deltascope rules explain ddl.table.comment.require --format json
```

### Filters (`rules list`)

| Flag | Description |
|------|-------------|
| `--dialect` | Filter by dialect (`mysql`, `tidb`, `postgresql`, `common`) |
| `--level` | Filter by default level (`blocker`, `warning`, `notice`) |
| `--kind` | Filter by statement kind (`ddl`, `dml`) |
| `--category` | Case-insensitive category/family filter |
| `--search` | Case-insensitive search across rule ID, summary, risk, suggestion, config key, and tags |
| `--format` | Output format: `text` (default) or `json` |
| `--limit` | Max rows to display (`0` = unlimited) |

### `level` Vocabulary

Rules use `level` — not `severity` — as the public finding field. Values are `blocker`, `warning`, and `notice`. This vocabulary is unchanged from earlier releases and matches both finding JSON output and rule configuration in `deltascope.example.yaml`. There is no `severity` field anywhere in the public output.

## Rule Catalog Facts

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

## Non-Goals

- No new audit rules.
- No rule evaluation behavior changes.
- No finding JSON shape changes.
- No `severity` field.
- No parser support changes.
- No SDK/HTTP/MCP rule discovery surfaces in this release.
- No change to v0.280.0 DDL coverage query behavior.
- No config file shape changes.
- No default rule level changes.

## DDL Coverage Census (unchanged)

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

### PostgreSQL ALTER TABLE Residual (unchanged)

`66/60/2/0/4/0`

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE config entries: **53**.
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).
- DDL coverage catalog: **400** entries (61 MySQL / 54 TiDB / 285 PostgreSQL / 18 parser_upgrade_candidate) (unchanged).
- Parser-error total: **29** cases across all dialects (unchanged).

## Decision Record

`docs/decisions/2026-06-11-v0.290.0-rule-discoverability.md`
