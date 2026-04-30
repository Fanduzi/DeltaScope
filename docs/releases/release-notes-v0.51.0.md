# DeltaScope v0.51.0 Release Notes

## Summary

v0.51.0 extends PostgreSQL ALTER TABLE audit coverage with three new gap-fill rules. DeltaScope now warns on `ALTER TABLE ... DROP COLUMN`, `ALTER TABLE ... VALIDATE CONSTRAINT`, and nullable `ALTER TABLE ... ADD COLUMN` — closing the most common ALTER TABLE safety gaps beyond the existing migration-safety rules.

## Added

- Three new PostgreSQL-only rules:
  - `ddl.pg.alter.drop_column.advisory` — warning when `ALTER TABLE ... DROP COLUMN` removes a column (warning)
  - `ddl.pg.alter.validate_constraint.advisory` — notice when `ALTER TABLE ... VALIDATE CONSTRAINT` runs a validation scan (notice)
  - `ddl.pg.alter.add_column.nullable.notice` — notice when `ALTER TABLE ... ADD COLUMN` adds a nullable column without a default (notice)
- Corpus fixtures covering each rule's trigger forms.
- Service-level tests through `AuditSQL` for all three rules.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- This is not full PostgreSQL ALTER TABLE coverage. Remaining ALTER TABLE sub-commands (e.g., `ALTER COLUMN TYPE`, `ADD CONSTRAINT ... NOT VALID`, `DISABLE TRIGGER`) remain explicit boundaries.
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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.51.0/install.sh | \
  DELTASCOPE_VERSION=v0.51.0 sh
```
