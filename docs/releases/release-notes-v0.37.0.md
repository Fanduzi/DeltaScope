# DeltaScope v0.37.0 Release Notes

## Summary

DeltaScope v0.37.0 adds PostgreSQL `CREATE TABLE` primary-key fact support. Inline, table-level, named, and composite primary-key declarations now populate DeltaScope's normalized primary-key contract, allowing existing primary-key rules to audit PostgreSQL `CREATE TABLE` statements.

## What Changed

### PostgreSQL Primary-Key Fact Extraction

The PostgreSQL extractor now populates the shared `DDL.PrimaryKey` contract for `CREATE TABLE` statements with the following forms:

| PostgreSQL Form | Example |
|----------------|---------|
| Inline | `CREATE TABLE t (id bigint PRIMARY KEY)` |
| Table-level | `CREATE TABLE t (id bigint, PRIMARY KEY (id))` |
| Named | `CREATE TABLE t (id bigint, CONSTRAINT t_pkey PRIMARY KEY (id))` |
| Composite | `CREATE TABLE t (a int, b int, PRIMARY KEY (a, b))` |

Primary-key columns are treated as effectively `NOT NULL` — consistent with PostgreSQL semantics where primary-key columns are always not-null regardless of the explicit `NOT NULL` clause.

### Rule Coverage Unlocked

Existing primary-key rules now apply to PostgreSQL `CREATE TABLE`:

| Rule ID | What It Flags |
|---------|---------------|
| `ddl.table.primary_key.bigint.require` | PostgreSQL primary-key column is not BIGINT |
| `ddl.table.primary_key.columns.max_count` | PostgreSQL composite primary key exceeds the configured column limit |

`ddl.table.primary_key.not_null.require` does not produce a stable negative case for PostgreSQL because PK columns are treated as effectively NOT NULL.

### Public Surfaces

All four product surfaces produce explicit `rule_id` findings for PostgreSQL primary-key violations:

| Surface | Behavior |
|---------|----------|
| CLI | Normal audit output with `rule_id` findings |
| HTTP (`POST /v1/audit`) | Normal audit response with `rule_id` findings |
| MCP (`audit_sql`) | Normal tool result with `rule_id` findings |
| `pkg/deltascope` | `Audit()` returns result with findings containing explicit `rule_id` |

## What Did Not Change

- No full PostgreSQL index support.
- No `ALTER TABLE ADD PRIMARY KEY` support.
- No live schema primary-key introspection.
- No new primary-key rule IDs.
- No full PostgreSQL constraint/index parity.
- No MySQL or TiDB behavior changes.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.37.0/install.sh | \
  DELTASCOPE_VERSION=v0.37.0 sh
```
