# DeltaScope v0.55.0 Release Notes

## Summary

v0.55.0 adds PostgreSQL type lifecycle coverage for enum types and type drops. DeltaScope now normalizes `CREATE TYPE ... AS ENUM`, `ALTER TYPE ... ADD VALUE`, and `DROP TYPE`, adds five PostgreSQL-only findings for enum and drop-type risk, and keeps composite types and domains as explicit unsupported boundaries.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE TYPE color AS ENUM ('red', 'green', 'blue')` | `create_type` (type_kind=enum, labels=red,green,blue) |
| `ALTER TYPE color ADD VALUE 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow) |
| `ALTER TYPE color ADD VALUE IF NOT EXISTS 'yellow'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, if_not_exists=true) |
| `ALTER TYPE color ADD VALUE 'yellow' BEFORE 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=before, neighbor=green) |
| `ALTER TYPE color ADD VALUE 'yellow' AFTER 'green'` | `alter_type` (type_kind=enum, action=add_value, value=yellow, placement=after, neighbor=green) |
| `DROP TYPE color` | `drop_type` |
| `DROP TYPE IF EXISTS color CASCADE` | `drop_type` (if_exists=true, cascade=true) |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.create_type.enum.notice` | `CREATE TYPE ... AS ENUM` | notice |
| `ddl.pg.alter_type.add_value.advisory` | `ALTER TYPE ... ADD VALUE` | warning |
| `ddl.pg.alter_type.add_value.position.notice` | `ALTER TYPE ... ADD VALUE ... BEFORE/AFTER` | notice |
| `ddl.pg.drop_type.advisory` | `DROP TYPE` | warning |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` | warning |

## Explicit Unsupported Boundaries

| SQL | Unsupported Feature |
|-----|-------------------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |
| `CREATE DOMAIN email AS text CHECK (...)` | `create_domain` |

## Test Coverage

- AST census tests documenting stable parser facts for all seven supported type lifecycle forms.
- Parser/extractor normalization tests for enum create, add value variants, and drop type variants.
- Corpus fixtures covering all five new rules' trigger forms.
- Service-level tests through `AuditSQL` for type lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- DeltaScope does not inspect live dependent objects.
- DeltaScope does not validate whether enum values are already used by data or application code.
- DeltaScope does not model full PostgreSQL type system semantics.
- This is not full PostgreSQL type lifecycle support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the five new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.55.0/install.sh | \
  DELTASCOPE_VERSION=v0.55.0 sh
```
