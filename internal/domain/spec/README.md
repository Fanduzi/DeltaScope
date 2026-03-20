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
- `AlterColumnChange`
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
  - `Name` is the canonical subject identifier:
    - existing-object actions use the pre-change name
    - pure-add actions use the created object name
    - table-option actions leave it empty
  - `Column` carries:
    - `OldName` for the existing source-side identifier when the statement names one
    - an optional target `Definition` reused from `Column`
    - rename intent is inferred from `OldName` plus `Definition.Name`, not a separate boolean
    - an optional `Change` block with statement-local relation facts only for semantics the statement explicitly spells out, such as nullability, default, and auto-increment
    - target type and unsigned shape still live on `Definition`, but are not separately labeled as touched change facts
  - `Index` carries `OldName` plus an optional target `Definition` reused from `Index`
  - `Options` is intentionally a flat normalized subset of table options, not a full option AST or ordering-preserving model

## Dependencies
- Upstream: application extraction and domain rule evaluation
- Downstream: none inside the domain core

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
