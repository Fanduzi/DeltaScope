# Domain Spec Module

Normalized statement specifications used as the stable input for rule evaluation.

## Files

| File | Responsibility |
|------|---------------|
| statement.go | Defines the top-level normalized statement model |
| ddl.go | Defines DDL-oriented specification types |
| dml.go | Defines DML-oriented specification types |

## Exports

- `Statement`
- `DDL`
- `Table`
- `Column`
- `Index`
- `Alter`
- `DML`

## Dependencies
- Upstream: parser extraction and domain rule evaluation
- Downstream: none inside the domain core

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
