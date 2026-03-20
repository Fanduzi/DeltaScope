# Domain Spec Module

Normalized statement specifications used as the stable input for rule evaluation.

## Files

| File | Responsibility |
|------|---------------|
| statement.go | Defines the top-level normalized statement model |
| statement_test.go | Verifies typed statement metadata behavior |
| ddl.go | Defines DDL-oriented specification types, including richer column facts, typed index metadata, and create-table shape flags for offline DDL rules |
| dml.go | Defines DML-oriented specification types, including operation metadata for rule applicability |

## Exports

- `Statement`
- `Kind`
- `Dialect`
- `DDL`
- `Table`
- `Column`
- `Constraint`
- `Index`
- `IndexKind`
- `AlterColumn`
- `AlterIndex`
- `Alter`
- `DML`
- `DMLOperation`

## Notes

- `Column` now carries offline-governance facts needed by column-focused DDL rules:
  - `Length`
  - `Unsigned`
  - `NotNull`
  - `AutoIncrement`
  - `HasDefault`
  - `DefaultValue`
  - `DefaultIsNull`
  - `DefaultIsCurrentTimestamp`
  - `OnUpdateCurrentTimestamp`
- `DDL` also carries create-table shape flags for:
  - `CREATE TABLE ... LIKE`
  - `CREATE TABLE ... AS SELECT`
  - partitioned tables
- `Alter` now has room for richer normalized payloads:
  - `Column` for column-oriented alter semantics
  - `Index` for index-oriented alter semantics
  - `Options` for table-option changes

## Dependencies
- Upstream: application extraction and domain rule evaluation
- Downstream: none inside the domain core

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
