# DeltaScope v0.48.0 Release Notes

## Summary

v0.48.0 ships the PostgreSQL DDL Coverage Census & Gap Closure Pack. A systematic audit of 56 representative PostgreSQL DDL forms through the full pipeline identified coverage gaps and closes them with four new PostgreSQL-only rules, an extractor fix for `ALTER TABLE ADD COLUMN` NOT NULL / DEFAULT constraints, and expanded SQL corpus coverage.

## Added

- Four new PostgreSQL-only DDL rules registered under `ddl.pg.*`:
  - `ddl.pg.drop_index.advisory` — emits a notice when `DROP INDEX` removes an index, advising review of dependent queries (default: `notice`).
  - `ddl.pg.alter.add_column.non_null_no_default.warn` — warns when `ALTER TABLE ADD COLUMN` adds a `NOT NULL` column without a `DEFAULT` clause, which can cause table rewrites on large tables (default: `warning`).
  - `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` — emits a notice when `ALTER TABLE ADD UNIQUE CONSTRAINT` does not include `NOT VALID` and no `CREATE UNIQUE INDEX CONCURRENTLY` follows, suggesting concurrent index creation for zero-downtime deployments (default: `notice`).
  - `ddl.pg.alter.drop_constraint.advisory` — emits a notice when `ALTER TABLE DROP CONSTRAINT` removes a CHECK, UNIQUE, or FOREIGN KEY constraint, advising review of dependent queries and data integrity implications (default: `notice`).
- PostgreSQL extractor fix: `CONSTR_NOTNULL` and `CONSTR_DEFAULT` constraints on `ALTER TABLE ADD COLUMN` now populate the column's `NotNull` and `Default` fields in the normalized spec, allowing rules that depend on these facts to evaluate correctly.
- SQL corpus expanded with new PostgreSQL DDL finding cases covering all four new rules.
- Census characterization test locking the 56-form inventory: total 56, parseable 56, classified 34, normalized 34, finding-covered 31, normalized-silent-pass 3, unsupported-explicit 22, parser-error 0.

## Changed

- Default policy now includes the four new `ddl.pg.*` rules for PostgreSQL dialect audits.
- DDL rule registration updated to include the new PostgreSQL-only rules in the `AppliesTo` gate.
- `hasColumnConstraint` helper added to DDL common rules for reusable column-constraint checks across PostgreSQL rules (internal-only, not a public export).

## Verification

- `make release-contract-gates VERSION=v0.48.0` — all gates pass
- `make sql-corpus-gates` — corpus coverage gates pass
- `make sql-corpus-report` — reports current supported-rule coverage inventory
- `go test ./... -count=1` — all unit tests pass

## Non-Goals

- No new MySQL or TiDB rule IDs, parser features, or policy changes.
- No changes to `CREATE INDEX CONCURRENTLY`, `ALTER TABLE VALIDATE CONSTRAINT`, or `ALTER TABLE DROP COLUMN` behavior — these remain normalized silent pass for this milestone.
- No public API contract changes. `hasColumnConstraint` is an internal helper, not an exported symbol.
- No release asset naming or npm launcher behavior changes.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.48.0/install.sh | \
  DELTASCOPE_VERSION=v0.48.0 sh
```
