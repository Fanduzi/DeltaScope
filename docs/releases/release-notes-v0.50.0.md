# DeltaScope v0.50.0 Release Notes

## Summary

v0.50.0 ships the PostgreSQL Object Lifecycle DDL Pack. DeltaScope now normalizes and audits PostgreSQL schema, sequence, and materialized view lifecycle DDL — `CREATE SCHEMA`, `DROP SCHEMA`, `CREATE SEQUENCE`, `ALTER SEQUENCE`, `DROP SEQUENCE`, `CREATE MATERIALIZED VIEW`, and `DROP MATERIALIZED VIEW` — with nine new PostgreSQL-only rules covering cascade drops, sequence cycling, and sequence restarts.

## Added

- PostgreSQL object lifecycle DDL normalization: schemas, sequences, and materialized views now pass through the audit pipeline instead of returning unsupported.
- Newly normalized operations:
  - `CREATE SCHEMA`
  - `DROP SCHEMA`
  - `CREATE SEQUENCE`
  - `ALTER SEQUENCE`
  - `DROP SEQUENCE`
  - `CREATE MATERIALIZED VIEW`
  - `DROP MATERIALIZED VIEW`
- Nine new PostgreSQL-only rules:
  - `ddl.pg.drop_schema.advisory` — notice when `DROP SCHEMA` removes a schema (notice)
  - `ddl.pg.drop_schema.cascade.warn` — warning when `DROP SCHEMA ... CASCADE` is used (warning)
  - `ddl.pg.create_sequence.cycle.warn` — warning when `CREATE SEQUENCE ... CYCLE` may cause sequence value wraparound (warning)
  - `ddl.pg.alter_sequence.restart.warn` — warning when `ALTER SEQUENCE ... RESTART` resets the sequence counter (warning)
  - `ddl.pg.alter_sequence.cycle.warn` — warning when `ALTER SEQUENCE ... CYCLE` enables value wraparound on an existing sequence (warning)
  - `ddl.pg.drop_sequence.advisory` — notice when `DROP SEQUENCE` removes a sequence (notice)
  - `ddl.pg.drop_sequence.cascade.warn` — warning when `DROP SEQUENCE ... CASCADE` is used (warning)
  - `ddl.pg.drop_materialized_view.advisory` — notice when `DROP MATERIALIZED VIEW` removes a materialized view (notice)
  - `ddl.pg.drop_materialized_view.cascade.warn` — warning when `DROP MATERIALIZED VIEW ... CASCADE` is used (warning)
- Service-level tests for all lifecycle operations through `AuditSQL`.
- Corpus fixtures for schema, sequence, and materialized view lifecycle forms.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- `REFRESH MATERIALIZED VIEW` remains unsupported/deferred.
- This is not full PostgreSQL object lifecycle coverage. Remaining unsupported DDL forms (triggers, functions, extensions, etc.) remain explicit boundaries.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the nine new PostgreSQL-only rule entries.
- No parser grammar changes beyond the lifecycle DDL normalization widening.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.50.0/install.sh | \
  DELTASCOPE_VERSION=v0.50.0 sh
```
