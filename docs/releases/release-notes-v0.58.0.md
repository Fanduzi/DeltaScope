# DeltaScope v0.58.0 Release Notes

## Summary

v0.58.0 adds PostgreSQL composite type lifecycle narrow support. DeltaScope now normalizes `CREATE TYPE ... AS (...)` and `ALTER TYPE ... RENAME TO` / `ALTER TYPE ... SET SCHEMA` through the audit pipeline, adding three PostgreSQL-only findings for offline migration review. `DROP TYPE` intentionally reuses existing v0.55.0 type lifecycle rules (`ddl.pg.drop_type.advisory`, `ddl.pg.drop_type.cascade.warn`) — no new composite-specific DROP TYPE rule is introduced. Attribute-level operations (`ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, `RENAME ATTRIBUTE`) remain explicitly unsupported/deferred.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE qualified.address AS (street text, city text)` | `create_type_composite` |
| `CREATE TYPE address AS (street text COLLATE "C", city text)` | `create_type_composite` (collation noted but not interpreted) |
| `ALTER TYPE address RENAME TO mailing_address` | `alter_type` (action=rename) |
| `ALTER TYPE address SET SCHEMA archive` | `alter_type` (action=set_schema) |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.create_type.composite.notice` | `CREATE TYPE ... AS (...)` | notice |
| `ddl.pg.alter_type.composite_rename.notice` | `ALTER TYPE ... RENAME TO` | notice |
| `ddl.pg.alter_type.composite_set_schema.notice` | `ALTER TYPE ... SET SCHEMA` | notice |

## DROP TYPE: Existing Rules Reused

`DROP TYPE` statements for composite types intentionally reuse the existing v0.55.0 type lifecycle rules:

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.drop_type.advisory` | `DROP TYPE` | warning |
| `ddl.pg.drop_type.cascade.warn` | `DROP TYPE ... CASCADE` | warning |

No composite-specific DROP TYPE rule exists or is planned. The existing rules already provide adequate coverage.

## Explicit Unsupported/Deferred Boundaries

| SQL | Unsupported Feature |
|-----|-------------------|
| `ALTER TYPE ... ADD ATTRIBUTE` | `alter_type_add_attribute` |
| `ALTER TYPE ... DROP ATTRIBUTE` | `alter_type_drop_attribute` |
| `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE` | `alter_type_alter_attribute_type` |
| `ALTER TYPE ... RENAME ATTRIBUTE ... TO ...` | `alter_type_rename_attribute` |

## Collation and Type Semantics Boundary

DeltaScope recognizes `COLLATE` annotations inside composite type attribute definitions (e.g., `CREATE TYPE address AS (street text COLLATE "C", city text)`) at the structural level but does not render, interpret, or validate collation semantics. This is an intentional design decision.

## Test Coverage

- AST census tests documenting stable parser facts for all composite type lifecycle forms.
- Parser/extractor normalization tests for all supported composite DDL variants.
- Corpus fixtures covering all three new rules' trigger forms.
- Service-level tests through `AuditSQL` for representative composite lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- DeltaScope does not perform live dependency validation on composite types.
- DeltaScope does not model full PostgreSQL type system semantics.
- This is narrow composite type lifecycle support — not complete PostgreSQL type system support.
- Attribute-level operations (`ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, `RENAME ATTRIBUTE`) remain explicitly deferred.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.58.0/install.sh | \
  DELTASCOPE_VERSION=v0.58.0 sh
```
