# DeltaScope v0.280.0 Release Notes

## Summary — DDL Coverage Catalog Query

v0.280.0 adds a `deltascope ddl-coverage` CLI command that queries the DDL coverage catalog introduced in v0.270.0. Users can filter by dialect, classification, guidance code, family, form, or free-text search, and output results as a human-readable table or JSON. This release does **not** add parser support, change parser behavior, add SQL audit rules, implement a fallback parser, reduce `parser_error` counts, change audit verdict or finding semantics, or alter SQL audit behavior in any way.

## CLI Catalog Lookup

The new `deltascope ddl-coverage` command provides interactive access to the coverage catalog:

```bash
# List all entries (default table format, capped at 50 rows)
deltascope ddl-coverage

# Filter by dialect
deltascope ddl-coverage --dialect mysql

# Filter by classification
deltascope ddl-coverage --classification finding_covered

# Filter by guidance code
deltascope ddl-coverage --guidance-code parser_upgrade_candidate

# Free-text search across family and form labels
deltascope ddl-coverage --search "ALTER TABLE"

# JSON output for scripting
deltascope ddl-coverage --format json --limit 0
```

### Filters

| Flag | Description |
|------|-------------|
| `--dialect` | Filter by dialect (`mysql`, `tidb`, `postgresql`) |
| `--classification` | Filter by classification (`finding_covered`, `normalized_silent`, `unsupported_boundary`, `parser_error`) |
| `--guidance-code` | Filter by guidance code (e.g. `parser_upgrade_candidate`) |
| `--family` | Filter by DDL family (e.g. `ALTER TABLE`) |
| `--form` | Filter by form label |
| `--search` | Case-insensitive substring search across family and form |
| `--format` | Output format: `table` (default) or `json` |
| `--limit` | Max rows to display (`0` = unlimited) |

## Non-Goals

- No parser support added.
- No fallback parser.
- No new SQL audit rules.
- No parser_error count reduction.
- No audit verdict or finding semantic changes.
- No SQL audit behavior change.
- No runtime behavior changes.
- No npm/Homebrew behavior change.

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
- MySQL/TiDB DDL Notice section: **27** (unchanged).
- TiDB-Specific subsection: **7** (unchanged).

## Decision Record

`docs/decisions/2026-06-07-v0.280.0-ddl-coverage-catalog-query.md`
