# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Next Milestone: v0.25.0 SQL Corpus & Boundary Confidence Pack

**Goal:** build a durable SQL corpus and table-driven audit harness for MySQL, TiDB, and PostgreSQL.

The milestone should answer a practical release-confidence question: which representative SQL statements have actually been run through DeltaScope, and what outcomes are expected?

### Scope

- Add `testdata/sql-corpus/` with dialect-specific SQL examples and expected outcomes.
- Cover representative MySQL, TiDB, and PostgreSQL DDL/DML cases.
- Assert stable audit facts instead of full rendered-output snapshots.
- Include supported, unsupported, finding-producing, clean, and boundary examples.
- Run corpus tests through the existing audit application layer.
- Keep transport E2E coverage sampled rather than exhaustive.

### Initial Coverage Targets

- MySQL DDL/DML baseline: `CREATE TABLE`, `ALTER TABLE`, `CREATE/DROP INDEX`, `UPDATE`, `DELETE`, and `INSERT`.
- TiDB DDL/DML baseline: MySQL-compatible core cases plus TiDB-specific compatibility boundaries where relevant.
- PostgreSQL DDL baseline: recently expanded `ALTER TABLE` and `CREATE TABLE` forms from the `v0.21.0` through `v0.24.0` release line.

### Non-Goals

- Do not claim full vendor grammar conformance.
- Do not add a broad new rule pack.
- Do not convert every case into CLI/HTTP/MCP E2E.
- Do not widen SQL support just to populate the corpus.

## Explicit Follow-Up TODO

### PostgreSQL CREATE TABLE Unsupported Boundary Pack

This is intentionally separate from `v0.25.0`. The SQL corpus should record current boundary behavior, but this follow-up pack should own the actual PostgreSQL unsupported-boundary fixes and parity work.

Track these cases:

- `GENERATED ... AS IDENTITY`
- generated stored columns
- partitioned `CREATE TABLE`
- exclusion constraints
- schema-qualified foreign-key references

Known implementation concern:

- The current PostgreSQL extractor has a `column.GetIdentity()` guard, but `pg_query_go/v6` may expose identity columns as a `CONSTR_IDENTITY` constraint instead. This needs a dedicated red/green investigation before changing behavior.

Expected follow-up outcomes:

- Each boundary has parser-level tests.
- Each boundary has audit-service tests.
- At least CLI and public Go API unsupported output parity is covered.
- Docs distinguish unsupported boundaries from supported `v0.23.0` / `v0.24.0` create-table semantics.

