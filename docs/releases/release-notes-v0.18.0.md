# DeltaScope v0.18.0 Release Notes

Release date: 2026-04-09

## Overview

DeltaScope `v0.18.0` adds **PostgreSQL metadata-aware audit** with full transport parity across CLI, HTTP, and MCP interfaces. This is the first release where DeltaScope can connect to a live PostgreSQL instance, retrieve schema metadata, run `EXPLAIN` for DML impact estimation, and evaluate rules against real database state — matching the existing MySQL/TiDB metadata-aware experience.

## What's Changed

### PostgreSQL Metadata-Aware Audit

DeltaScope now supports metadata-aware audit for PostgreSQL 12+ alongside the existing MySQL and TiDB support:

- **Schema resolution**: qualified table names in SQL (`app.users`) are parsed automatically; unqualified names resolve via `--schema` flag or unique-match inference across accessible schemas
- **Instance facts**: retrieves PostgreSQL version, database name, and schema catalog from `pg_catalog`
- **Table snapshots**: column definitions, constraints, primary keys, unique constraints, and indexes from `information_schema` and `pg_indexes`
- **DML impact estimation**: uses PostgreSQL `EXPLAIN` (not `EXPLAIN ANALYZE`) to estimate affected rows for `UPDATE` and `DELETE` statements — conservative, read-only, never executes the DML

### Transport Parity

All three transport surfaces now support PostgreSQL metadata-aware audit:

- **CLI**: `deltascope audit --dialect postgresql --host ... --port 5432 --user ...`
- **HTTP**: `POST /v1/audit` with `"dialect": "postgresql"` and a `connection` block
- **MCP**: `audit_sql` tool with `"dialect": "postgresql"` in the connection input

### New Metadata-Aware Rules for PostgreSQL

PostgreSQL-specific rule coverage:

- `ddl.alter.drop_primary_key.forbid` — detects `DROP CONSTRAINT` on primary keys via `pg_constraint` mapping
- `ddl.alter.rename_column.exists.require` — verifies column existence before rename
- `ddl.alter.rename_index.forbid` — flags index renames using `pg_indexes` owner resolution
- `ddl.alter.drop_column.exists.require` — verifies column existence before drop
- `ddl.table.exists.create.forbid` — checks table existence via metadata before `CREATE TABLE`

### E2E Test Coverage

Full end-to-end test suites for all three transports against a real PostgreSQL 17 container:

- CLI: 9 test cases via shell harness (`test_cli_metadata_e2e_postgresql.sh`)
- HTTP: 9 subtests via Go `e2e && postgresql` build tags
- MCP: 9 subtests via Go `e2e && postgresql` build tags

### Documentation

All reference and concept docs updated to reflect PostgreSQL metadata-aware support, including CLI flags, HTTP API examples, MCP usage, capability matrix, and troubleshooting guide.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.18.0/install.sh | \
  DELTASCOPE_VERSION=v0.18.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## Compatibility

No breaking changes. `v0.18.0` extends the existing audit contract with additive PostgreSQL metadata-aware capabilities:

- All existing MySQL/TiDB offline and metadata-aware behavior is unchanged
- PostgreSQL offline audit continues to work as before
- New metadata-aware mode is opt-in via connection flags
- `drop_primary_key` rule now fires for PostgreSQL `ALTER TABLE ... DROP CONSTRAINT` on primary keys
