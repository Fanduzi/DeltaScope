# DeltaScope v0.64.0 Release Notes

## Summary

v0.64.0 adds cross-dialect DDL coverage census and closes the first database/schema lifecycle parity gap. MySQL and TiDB `CREATE DATABASE`, `CREATE SCHEMA`, `DROP DATABASE`, and `DROP SCHEMA` now produce explicit audit findings instead of passing silently. PostgreSQL `CREATE SCHEMA` and `CREATE SCHEMA IF NOT EXISTS` now produce an explicit notice. Three new audit rules, SQL corpus expansion to 405/405 targets, and full public surface coverage across CLI, HTTP, MCP, and SDK.

## Cross-Dialect DDL Census

v0.64.0 starts with a systematic census of representative DDL forms across MySQL, TiDB, and PostgreSQL. Each form is classified as `parser_error`, `unsupported_boundary`, `normalized_silent`, or `finding_covered`. The census makes it visible where each dialect stands for table lifecycle, view lifecycle, database/schema lifecycle, index lifecycle, constraint lifecycle, trigger lifecycle, routine lifecycle, and privilege/DCL lifecycle.

## New Rules

| Rule ID | Dialects | Level | Triggers |
|---------|----------|:------:|----------|
| `ddl.database.create.notice` | MySQL/TiDB | notice | `CREATE DATABASE`, `CREATE DATABASE IF NOT EXISTS`, `CREATE SCHEMA` |
| `ddl.database.drop.warn` | MySQL/TiDB | warning | `DROP DATABASE`, `DROP DATABASE IF EXISTS`, `DROP SCHEMA` |
| `ddl.pg.create_schema.notice` | PostgreSQL | notice | `CREATE SCHEMA`, `CREATE SCHEMA IF NOT EXISTS` |

### MySQL/TiDB Database/Schema Lifecycle

In MySQL and TiDB, `SCHEMA` is a synonym for `DATABASE`. Both forms trigger the same rule:

```bash
deltascope audit --sql "create database app"
# → [notice] ddl.database.create.notice

deltascope audit --sql "drop database app"
# → [warning] ddl.database.drop.warn

deltascope audit --sql "create schema app"
# → [notice] ddl.database.create.notice

deltascope audit --sql "drop schema app"
# → [warning] ddl.database.drop.warn
```

`IF NOT EXISTS` and `IF EXISTS` variants still emit findings.

### PostgreSQL CREATE SCHEMA

PostgreSQL `CREATE SCHEMA` now emits a notice. Existing `DROP SCHEMA` rules (`ddl.pg.drop_schema.advisory`, `ddl.pg.drop_schema.cascade.warn`) are unchanged.

```bash
deltascope audit --dialect postgresql --sql "create schema app"
# → [notice] ddl.pg.create_schema.notice
```

## Normalization

| Dialect | Statement | Normalized Operation | Object Type |
|---------|-----------|---------------------|-------------|
| MySQL/TiDB | `CREATE DATABASE app` | `create_schema` | `database` |
| MySQL/TiDB | `CREATE SCHEMA app` | `create_schema` | `database` |
| MySQL/TiDB | `DROP DATABASE app` | `drop_schema` | `database` |
| MySQL/TiDB | `DROP SCHEMA app` | `drop_schema` | `database` |
| PostgreSQL | `CREATE SCHEMA app` | `create_schema` | `schema` |

MySQL/TiDB `CREATE DATABASE ... CHARACTER SET`/`COLLATE` options are preserved as parser facts but no policy rule governs them.

## Quality

- SQL corpus: 214 policy rules, 405/405 supported targets, 100% coverage
- Public surface coverage verified across CLI, HTTP, MCP, and SDK
- AST characterization tests documenting stable parser facts for database/schema lifecycle forms
- Default policy dialect isolation: MySQL/TiDB audits do not emit `ddl.pg.*`; PostgreSQL audits do not emit `ddl.database.*`

## Non-Goals

- No full DDL support claim.
- No trigger lifecycle support in this milestone.
- No routine/function/procedure lifecycle support in this milestone.
- No event lifecycle support.
- No database privilege/DCL parity.
- No live database/schema existence validation.
- No charset/collation/tablespace/owner validation.
- PostgreSQL `CREATE SCHEMA AUTHORIZATION` and nested `CREATE SCHEMA ... CREATE TABLE ...` remain unsupported/deferred.
- Existing PostgreSQL `DROP SCHEMA` behavior is unchanged.
- DeltaScope does not execute migrations.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.64.0/install.sh | \
  DELTASCOPE_VERSION=v0.64.0 sh
```

## Upgrade

If you previously installed v0.63.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.64.0/install.sh | \
  DELTASCOPE_VERSION=v0.64.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.64.0

deltascope audit --sql "create database app"
# Should produce ddl.database.create.notice finding

deltascope audit --dialect postgresql --sql "create schema app"
# Should produce ddl.pg.create_schema.notice finding
```
