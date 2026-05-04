# DeltaScope v0.57.0 Release Notes

## Summary

v0.57.0 adds PostgreSQL domain lifecycle coverage. DeltaScope now normalizes `CREATE DOMAIN`, `ALTER DOMAIN` (constraint, default, not null, rename), and `DROP DOMAIN` through the audit pipeline, adding seven PostgreSQL-only findings for offline migration review. `CHECK` and `DEFAULT` expression text is intentionally not rendered — rules emit boolean facts (`has_check`, `has_default`) and constraint names where available.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `CREATE DOMAIN email AS text CHECK (VALUE <> '')` | `create_domain` |
| `CREATE DOMAIN email AS text NOT NULL DEFAULT 'n/a' CONSTRAINT chk CHECK (...)` | `create_domain` |
| `ALTER DOMAIN email SET DEFAULT 'unknown@example.com'` | `alter_domain` (action=set_default) |
| `ALTER DOMAIN email DROP DEFAULT` | `alter_domain` (action=drop_default) |
| `ALTER DOMAIN email SET NOT NULL` | `alter_domain` (action=set_not_null) |
| `ALTER DOMAIN email DROP NOT NULL` | `alter_domain` (action=drop_not_null) |
| `ALTER DOMAIN email ADD CONSTRAINT email_not_empty CHECK (...)` | `alter_domain` (action=add_constraint) |
| `ALTER DOMAIN email DROP CONSTRAINT email_not_empty` | `alter_domain` (action=drop_constraint) |
| `ALTER DOMAIN email VALIDATE CONSTRAINT email_not_empty` | `alter_domain` (action=validate_constraint) |
| `ALTER DOMAIN email RENAME TO contact_email` | `alter_domain` (action=rename) |
| `DROP DOMAIN email` | `drop_domain` |
| `DROP DOMAIN IF EXISTS email CASCADE` | `drop_domain` |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.create_domain.notice` | `CREATE DOMAIN` | notice |
| `ddl.pg.alter_domain.constraint.notice` | `ALTER DOMAIN ... ADD/DROP/VALIDATE CONSTRAINT` | notice |
| `ddl.pg.alter_domain.default.notice` | `ALTER DOMAIN ... SET/DROP DEFAULT` | notice |
| `ddl.pg.alter_domain.not_null.notice` | `ALTER DOMAIN ... SET/DROP NOT NULL` | notice |
| `ddl.pg.alter_domain.rename.notice` | `ALTER DOMAIN ... RENAME TO` | notice |
| `ddl.pg.drop_domain.advisory` | `DROP DOMAIN` | warning |
| `ddl.pg.drop_domain.cascade.warn` | `DROP DOMAIN ... CASCADE` | warning |

> `DROP DOMAIN IF EXISTS ... CASCADE` intentionally emits two findings: `ddl.pg.drop_domain.advisory` and `ddl.pg.drop_domain.cascade.warn`.

## Explicit Unsupported Boundaries

| SQL | Unsupported Feature |
|-----|-------------------|
| `CREATE TYPE address AS (street text, city text)` | `create_type_composite` |

## Expression Rendering Boundary

DeltaScope does not render `CHECK` or `DEFAULT` expression text. Rules expose boolean facts (`has_check`, `has_default`, `not_null`) and constraint names where available, but never the expression body. This is an intentional design decision to avoid false specificity in offline review.

## Test Coverage

- AST census tests documenting stable parser facts for all 15 domain lifecycle forms.
- Parser/extractor normalization tests for all supported domain DDL variants.
- Corpus fixtures covering all seven new rules' trigger forms.
- Service-level tests through `AuditSQL` for 12 domain lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- DeltaScope does not render `CHECK` or `DEFAULT` expression text.
- DeltaScope does not perform live dependency validation on domains.
- `CREATE TYPE ... AS (...)` composite types remain explicitly unsupported as `create_type_composite`.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the seven new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.57.0/install.sh | \
  DELTASCOPE_VERSION=v0.57.0 sh
```
