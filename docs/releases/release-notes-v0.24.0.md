# DeltaScope v0.24.0 Release Notes

Release date: 2026-04-11

## Overview

DeltaScope `v0.24.0` is the **PostgreSQL CREATE TABLE Semantics Pack**. It deepens the semantic value of the PostgreSQL create-table shapes that `v0.23.0` brought into the shared audit pipeline, rather than widening syntax coverage or adding new rule IDs.

This release does not claim full PostgreSQL DDL support. It also does not add new PostgreSQL rule IDs, new severity levels, new CLI flags, new HTTP payload contracts, new MCP tool contracts, or new public Go API contracts.

## What's Changed

### Richer PostgreSQL Foreign-Key Semantics

PostgreSQL `CREATE TABLE` foreign-key structures now preserve more shared semantic detail:

- `ReferencedTable` — the parser-extracted referenced table name from `REFERENCES table(column)` forms
- `ReferencedColumns` — the parser-extracted referenced column names

Both named `FOREIGN KEY ... REFERENCES` and column-level inline `REFERENCES` carry these facts through the shared `spec.Constraint` model. These are **parser-owned shared contract facts**, not live metadata truth. They represent what the SQL statement declares, not what the database schema currently contains.

### Shared Rule Reuse Continues

This is a semantics deepening, not a new rule pack.

- Existing shared naming governance (`ddl.constraint.foreign_key.name.*`) continues to apply on named `FOREIGN KEY` constraints with the richer shape.
- Existing `ddl.table.foreign_key.forbid` continues to fire for all foreign-key forms, including inline `REFERENCES` carrying the richer semantics.
- FK naming rules remain suppressed when FK-forbid is active (the default policy).
- No new rule configuration items are needed.

### Unsupported Boundaries

Adjacent unsupported forms remain explicitly outside the supported surface:

- `GENERATED ... AS IDENTITY` — still unsupported in `CREATE TABLE` column definitions
- `CREATE TABLE ... PARTITION BY` — still unsupported
- `CREATE OR REPLACE VIEW` — still unsupported

These boundaries are now locked by explicit tests on the service layer and public Go API.

### Surface Parity

The richer PostgreSQL foreign-key semantics are confirmed across:

- `deltascope` CLI
- HTTP `POST /v1/audit`
- MCP `audit_sql`
- public Go API `pkg/deltascope`

## Compatibility

No breaking changes.

- Existing MySQL, TiDB, and PostgreSQL audit behavior remains compatible.
- No new rule IDs, severity levels, or trigger conditions are introduced.
- CLI, HTTP, MCP, and `pkg/deltascope` public contracts remain unchanged. The `ReferencedTable` and `ReferencedColumns` fields are additive and use `omitempty` JSON encoding.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.24.0/install.sh | \
  DELTASCOPE_VERSION=v0.24.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
