# DeltaScope v0.39.0 Release Notes

## Summary

DeltaScope v0.39.0 preserves statement-local primary-key and unique constraint facts for approved `ALTER TABLE ... ADD CONSTRAINT` forms, allowing existing primary-key and unique/index rules to produce findings across CLI, HTTP, MCP, and `pkg/deltascope` surfaces.

## What Changed

### PostgreSQL ALTER TABLE Constraint Fact Support

Existing primary-key and unique/index rules now apply to approved PostgreSQL `ALTER TABLE ... ADD CONSTRAINT` forms:

| PostgreSQL Form | Example |
|----------------|---------|
| Inline primary key | `ALTER TABLE users ADD PRIMARY KEY (id)` |
| Named primary key | `ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id)` |
| Inline unique | `ALTER TABLE users ADD UNIQUE (email)` |
| Named unique | `ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)` |

### Rule Coverage Unlocked

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.table.primary_key.bigint.require` | Primary-key column is not BIGINT |
| `ddl.table.primary_key.columns.max_count` | Composite primary key exceeds the configured column limit |
| `ddl.alter.add_index.unique.prefix.require` | Unique constraint name does not start with the required prefix (default: `uniq_`) |

### Public Surfaces

All four product surfaces produce explicit `rule_id` findings:

| Surface | Behavior |
|---------|----------|
| CLI | Normal audit output with `rule_id` findings |
| HTTP (`POST /v1/audit`) | Normal audit response with `rule_id` findings |
| MCP (`audit_sql`) | Normal tool result with `rule_id` findings |
| `pkg/deltascope` | `Audit()` returns result with findings containing explicit `rule_id` |

### Docker-backed E2E Coverage

PostgreSQL CLI e2e covers `ddl.alter.add_index.unique.prefix.require` for statement-local `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` audit through the Docker-backed test path.

## What Did Not Change

- No full `ALTER TABLE ADD CONSTRAINT` support — foreign keys, check constraints, and exclusion constraints remain out of scope.
- No deferrable constraint support.
- No constraint validation lifecycle support (`VALIDATE CONSTRAINT`, `NOT VALID`).
- No partial or expression index support.
- No operator class support.
- No live schema reconstruction from constraints.
- No new rule IDs — existing rules cover approved forms through extended applicability and projection helpers.
- No MySQL or TiDB behavior changes.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.39.0/install.sh | \
  DELTASCOPE_VERSION=v0.39.0 sh
```
