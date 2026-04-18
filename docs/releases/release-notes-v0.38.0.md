# DeltaScope v0.38.0 Release Notes

## Summary

DeltaScope v0.38.0 extends PostgreSQL unique/index audit coverage for statement-local unique constraints and simple btree `CREATE INDEX` forms. Existing index rules now produce findings for the approved PostgreSQL forms, with corpus, public-surface, and Docker-backed e2e coverage.

## What Changed

### PostgreSQL Unique/Index Rule Coverage

Existing generic index rules now apply to standalone PostgreSQL `CREATE INDEX` and `CREATE UNIQUE INDEX` statements:

| PostgreSQL Form | Example |
|----------------|---------|
| Secondary index | `CREATE INDEX idx_users_email ON users (email)` |
| Unique index | `CREATE UNIQUE INDEX uniq_users_email ON users (email)` |
| Concurrent build | `CREATE INDEX CONCURRENTLY idx_users_email ON users (email)` |
| Inline UNIQUE constraint | `CREATE TABLE t (email text UNIQUE)` |

### Rule Coverage Unlocked

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.index.secondary.prefix.require` | Secondary index name does not start with the required prefix (default: `idx_`) |
| `ddl.index.unique.prefix.require` | Unique index name does not start with the required prefix (default: `uniq_`) |
| `ddl.index.columns.max_count` | Index spans more columns than the allowed maximum |

### Public Surfaces

All four product surfaces produce explicit `rule_id` findings:

| Surface | Behavior |
|---------|----------|
| CLI | Normal audit output with `rule_id` findings |
| HTTP (`POST /v1/audit`) | Normal audit response with `rule_id` findings |
| MCP (`audit_sql`) | Normal tool result with `rule_id` findings |
| `pkg/deltascope` | `Audit()` returns result with findings containing explicit `rule_id` |

### Docker-backed E2E Coverage

PostgreSQL CLI e2e covers `ddl.index.unique.prefix.require` for statement-local `CREATE UNIQUE INDEX` audit through the Docker-backed test path.

## What Did Not Change

- No full PostgreSQL index support.
- No partial index support (`WHERE` clause).
- No expression index support (`((lower(email)))`).
- No INCLUDE clause support.
- No operator class support.
- No non-btree access method support (`USING hash`, etc.).
- No NULLS NOT DISTINCT support.
- No live schema index introspection.
- No new index rule IDs.
- No MySQL or TiDB behavior changes.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.38.0/install.sh | \
  DELTASCOPE_VERSION=v0.38.0 sh
```
