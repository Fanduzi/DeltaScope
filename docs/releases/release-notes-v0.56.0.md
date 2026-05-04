# DeltaScope v0.56.0 Release Notes

## Summary

v0.56.0 adds PostgreSQL ALTER TABLE logged-state coverage and improves ALTER COLUMN TYPE USING metadata extraction. DeltaScope now normalizes `ALTER TABLE ... SET LOGGED` and `ALTER TABLE ... SET UNLOGGED`, adds two PostgreSQL-only findings for logged-state transitions, and captures the USING expression in ALTER COLUMN TYPE normalization. SET TABLESPACE remains an explicit unsupported boundary.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `ALTER TABLE users SET LOGGED` | `alter` (action=set_logged) |
| `ALTER TABLE users SET UNLOGGED` | `alter` (action=set_unlogged) |
| `ALTER TABLE users ALTER COLUMN name TYPE varchar(100) USING name::varchar(100)` | `alter` (action=alter_column_type, using=name::varchar(100)) |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.alter.set_logged.notice` | `ALTER TABLE ... SET LOGGED` | notice |
| `ddl.pg.alter.set_unlogged.notice` | `ALTER TABLE ... SET UNLOGGED` | notice |

## Improvements

- `ALTER TABLE ... ALTER COLUMN TYPE ... USING ...` now captures the USING expression in the normalized alter metadata, making review of unsafe casts visible in audit output.

## Explicit Unsupported Boundaries

| SQL | Unsupported Feature |
|-----|-------------------|
| `ALTER TABLE users SET TABLESPACE pg_default` | `alter_set_tablespace` |

## Test Coverage

- AST census tests documenting stable parser facts for logged-state forms.
- Parser/extractor normalization tests for SET LOGGED, SET UNLOGGED, and ALTER COLUMN TYPE USING variants.
- Corpus fixtures covering both new rules' trigger forms.
- Service-level tests through `AuditSQL` for logged-state variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- DeltaScope does not verify whether the target table is currently logged or unlogged.
- DeltaScope does not evaluate WAL or replication implications of logged-state transitions.
- This is not full PostgreSQL ALTER TABLE grammar support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the two new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.56.0/install.sh | \
  DELTASCOPE_VERSION=v0.56.0 sh
```
