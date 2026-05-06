# DeltaScope v0.59.0 Release Notes

## Summary

v0.59.0 adds PostgreSQL extension lifecycle narrow support. DeltaScope now normalizes `CREATE EXTENSION`, `ALTER EXTENSION` (`UPDATE`, `UPDATE TO`, `SET SCHEMA`), and `DROP EXTENSION` through the audit pipeline, adding six PostgreSQL-only findings for offline migration review. Extension member mutation (`ALTER EXTENSION ... ADD/DROP TABLE`) remains explicitly unsupported/deferred. No live validation of extension availability, installed packages, version compatibility, or dependency graphs is performed.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE EXTENSION pg_trgm` | `create_extension` |
| `CREATE EXTENSION IF NOT EXISTS pg_trgm` | `create_extension` (if_not_exists=true) |
| `CREATE EXTENSION pg_trgm WITH SCHEMA utils` | `create_extension` (schema=utils) |
| `CREATE EXTENSION pg_trgm WITH VERSION '1.5'` | `create_extension` (version=1.5) |
| `CREATE EXTENSION pg_trgm CASCADE` | `create_extension` (cascade=true) |
| `ALTER EXTENSION pg_trgm UPDATE` | `alter_extension` (action=update) |
| `ALTER EXTENSION pg_trgm UPDATE TO '1.6'` | `alter_extension` (action=update_to) |
| `ALTER EXTENSION pg_trgm SET SCHEMA utils` | `alter_extension` (action=set_schema) |
| `DROP EXTENSION pg_trgm` | `drop_extension` |
| `DROP EXTENSION IF EXISTS pg_trgm` | `drop_extension` (if_exists=true) |
| `DROP EXTENSION pg_trgm CASCADE` | `drop_extension` (cascade=true) |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.create_extension.notice` | `CREATE EXTENSION` | notice |
| `ddl.pg.create_extension.cascade.warn` | `CREATE EXTENSION ... CASCADE` | warning |
| `ddl.pg.alter_extension.update.notice` | `ALTER EXTENSION ... UPDATE` / `UPDATE TO` | notice |
| `ddl.pg.alter_extension.set_schema.notice` | `ALTER EXTENSION ... SET SCHEMA` | notice |
| `ddl.pg.drop_extension.advisory` | `DROP EXTENSION` | warning |
| `ddl.pg.drop_extension.cascade.warn` | `DROP EXTENSION ... CASCADE` | warning |

## CASCADE Duplicate Findings

`CREATE EXTENSION ... CASCADE` triggers both `ddl.pg.create_extension.notice` and `ddl.pg.create_extension.cascade.warn`. `DROP EXTENSION ... CASCADE` triggers both `ddl.pg.drop_extension.advisory` and `ddl.pg.drop_extension.cascade.warn`. These duplicate findings are intentional — each rule addresses a distinct concern (the operation itself vs. the cascade side-effect risk).

## Explicit Unsupported/Deferred Boundaries

| SQL | Unsupported Feature |
|-----|-------------------|
| `ALTER EXTENSION ... ADD TABLE` | `alter_extension_add_member` |
| `ALTER EXTENSION ... DROP TABLE` | `alter_extension_drop_member` |

## Live Validation Boundary

DeltaScope does not perform live validation of any kind for extensions:
- No extension availability or installed-package checks
- No version compatibility validation
- No dependency graph resolution
- No schema existence verification

## Test Coverage

- AST census tests documenting stable parser facts for all extension lifecycle forms.
- Parser/extractor normalization tests for all supported extension DDL variants.
- Corpus fixtures covering all six new rules' trigger forms.
- Service-level tests through `AuditSQL` for representative extension lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- DeltaScope does not perform live dependency validation on extensions.
- DeltaScope does not model full PostgreSQL extension system semantics.
- This is narrow extension lifecycle support — not broad governance or admin DDL support.
- Extension member mutation (`ADD/DROP TABLE`) remains explicitly deferred.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the six new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.59.0/install.sh | \
  DELTASCOPE_VERSION=v0.59.0 sh
```
