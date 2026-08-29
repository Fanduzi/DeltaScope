# Decision: Normalize MySQL/TiDB Column-Level Primary Keys

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #42
Related commits: a88577f
Related tests:
- `TestSQLCorpusMySQLAndTiDB`
- `testdata/sql-corpus/mysql/ddl/findings/column_primary_key.expected.yaml`
- `testdata/sql-corpus/tidb/ddl/findings/column_primary_key.expected.yaml`
Related docs:
- `internal/infrastructure/parser/tidb/README.md`

## Context

The shared TiDB parser path already normalized table-level `PRIMARY KEY`
constraints into `spec.DDL.PrimaryKey`. It extracted column attributes such as
`UNSIGNED`, `NOT NULL`, and `AUTO_INCREMENT`, but discarded a column-level
`PRIMARY KEY` option. The existing primary-key rules therefore reported
`ddl.table.primary_key.require` for valid MySQL and TiDB `CREATE TABLE`
statements.

## Decision

In `extractCreateTable`, inspect each column's AST options after extracting the
column with the existing `extractColumn` path. When the option is exactly
`ast.ColumnOptionPrimaryKey`, populate `DDL.PrimaryKey` with the same
`spec.Index` shape used by table-level primary keys: name `primary`, primary
kind, and the extracted column name.

Column-level `UNIQUE` remains an index option and never populates
`DDL.PrimaryKey`. Table-level primary-key constraints continue through their
existing normalization loop.

## Rationale

The normalized `DDL.PrimaryKey` field is already the rule seam and the
column extractor already preserves the required attribute facts. Inline
primary keys also carry MySQL/TiDB's implicit `NOT NULL` semantics into the
normalized column. Adding the missing AST-to-spec mapping at the shared CREATE
TABLE extraction point keeps MySQL and TiDB behavior aligned without changing
policy, rule logic, or the public report shape.

## Public Contract

- MySQL and TiDB `CREATE TABLE` statements with a column-level `PRIMARY KEY`
  expose primary-key metadata to the existing DDL rule family.
- A single inline primary key satisfies `ddl.table.primary_key.require` and
  its extracted column attributes, including implicit `NOT NULL`, remain
  available to the existing semantic rules.
- A table with no primary key, including one with only a column-level
  `UNIQUE`, still receives `ddl.table.primary_key.require` when enabled.
- Table-level single and composite primary keys are unchanged.

## Deferred / Out Of Scope

- Changing primary-key policy levels, defaults, or rule implementations.
- Treating arbitrary `UNIQUE` constraints or indexes as primary keys.
- Expanding MySQL/TiDB CREATE TABLE grammar or inferring implicit keys.
- Changing PostgreSQL extraction or adding a new normalized-spec abstraction.

## Verification Evidence

The paired MySQL/TiDB corpus cases assert that the common
`BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY` form excludes the
primary-key presence and attribute findings. Paired controls assert that a
column-level `UNIQUE` without a primary key includes
`ddl.table.primary_key.require`. The existing `id BIGINT PRIMARY KEY` corpus
cases also assert that inline primary-key presence and implicit `NOT NULL` are
recognized.

## Consequences

Future MySQL/TiDB primary-key rule behavior should continue to consume
`DDL.PrimaryKey`; parser changes should preserve the distinction between
`ColumnOptionPrimaryKey` and `ColumnOptionUniqKey`.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/42
- Commit: `a88577f`
- Extractor: `internal/infrastructure/parser/tidb/extractor.go`
- Corpus: `testdata/sql-corpus/{mysql,tidb}/ddl/findings/`
