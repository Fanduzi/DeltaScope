# Domain Spec Module

Normalized statement specifications used as the stable input for rule evaluation.

## Files

| File | Responsibility |
|------|---------------|
| statement.go | Defines the top-level normalized statement model |
| statement_test.go | Verifies typed statement metadata behavior |
| ddl.go | Defines DDL-oriented specification types, including richer column facts such as length, nullability, and default/current-timestamp metadata for offline DDL rules |
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
- `Alter`
- `DML`
- `DMLOperation`

## Notes

- `Column` now carries offline-governance facts needed by column-focused DDL rules:
  - `Length`
  - `NotNull`
  - `HasDefault`
  - `DefaultValue`
  - `DefaultIsNull`
  - `DefaultIsCurrentTimestamp`
  - `OnUpdateCurrentTimestamp`

## Dependencies
- Upstream: application extraction and domain rule evaluation
- Downstream: none inside the domain core

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
