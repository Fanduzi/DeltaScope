# DeltaScope v0.19.0 Release Notes

Release date: 2026-04-09

## Overview

DeltaScope `v0.19.0` adds a **PostgreSQL Migration-Safety Rule Pack** and two new CI-native output formats (`github-actions` and `sarif`). The migration-safety rules guard against common PostgreSQL DDL patterns that can cause table rewrites, long-held locks, or production incidents — without requiring a database connection.

## What's Changed

### PostgreSQL Migration-Safety Rules (4 rules)

Four new offline-safe rules that flag high-risk PostgreSQL DDL patterns:

| Rule ID | What It Catches | Default Level |
|---------|----------------|:-------------:|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` without `CONCURRENTLY` holds an exclusive lock, blocking reads and writes | warning |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | Adding a `NOT NULL` column with a volatile default (e.g. `gen_random_uuid()`) triggers a full table rewrite | warning |
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK` constraint without `NOT VALID` requires a full table scan under `ACCESS EXCLUSIVE` lock | warning |
| `ddl.pg.alter.set_data_type.rewrite.warn` | Changing a column type (e.g. `varchar` to `integer`) may require a full table rewrite | warning |

These rules only apply when `--dialect postgresql` is set and are automatically skipped for MySQL/TiDB dialects.

### GitHub Actions Output Format

Use `--format github-actions` to produce inline CI annotations (`::error`, `::warning`, `::notice`) that render in the GitHub Actions workflow log. Special characters in titles and messages are escaped per the GitHub workflow command specification.

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

### SARIF Output Format

Use `--format sarif` to produce valid SARIF 2.1.0 JSON for GitHub Code Scanning, Azure DevOps, and other SARIF consumers. Rule metadata (help text from explanation suggestions) is placed under `tool.driver.rules`, and severity levels are mapped to SARIF levels.

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

### Rule Summary in Output

JSON, markdown, and quiet output now include a rule summary showing how many rules were loaded, how many were applicable to the given dialect, and how many were skipped. In JSON this appears as `rule_summary`; in markdown it renders as `## Rule Summary` and `## Skipped Rules` sections. GitHub Actions and SARIF output do not include rule summary.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.19.0/install.sh | \
  DELTASCOPE_VERSION=v0.19.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## Compatibility

No breaking changes. `v0.19.0` extends the existing audit contract with additive capabilities:

- All existing MySQL/TiDB/PostgreSQL offline and metadata-aware behavior is unchanged
- New PostgreSQL migration-safety rules are opt-in via `--dialect postgresql` (they are skipped for other dialects)
- New `github-actions` and `sarif` output formats are selected via `--format` and do not affect existing `markdown` or `json` output
- `rule_summary` is an additive field in JSON output; existing consumers that ignore unknown fields are unaffected
