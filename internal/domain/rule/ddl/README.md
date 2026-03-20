# Domain DDL Rule Module

Expanded create-table DDL rule catalog for table, column, audit-column, and index checks.

## Files

| File | Responsibility |
|------|---------------|
| common.go | Shared DDL rule IDs and create-table applicability checks |
| config.go | Parses policy params for DDL rule constructors |
| table_rules.go | Implements table comment and table name rules |
| primary_key_rules.go | Implements primary-key presence and column-count rules |
| column_rules.go | Implements table-column count and column-level governance rules |
| audit_column_rules.go | Implements audit timestamp column rules |
| index_rules.go | Implements create-table index count, prefix, and duplicate-index rules |
| register.go | Registers enabled DDL rules into the shared registry |
| table_rules_test.go | Verifies table comment and name-length rule behavior |
| primary_key_rules_test.go | Verifies primary-key requirement and shape rules |
| column_rules_test.go | Verifies column-count, comment, naming, default, nullability, and type rules |
| audit_column_rules_test.go | Verifies audit timestamp column rules |
| index_rules_test.go | Verifies create-table index governance rules |
| register_test.go | Verifies policy-backed DDL rule registration and deterministic ordering |

## Exports

- `Register(registry *rule.Registry, cfg policy.Policy) error`

## Dependencies
- Upstream: future application rule assembly and higher-level audit services
- Downstream: `internal/domain/policy`, `internal/domain/rule`, `internal/domain/spec`

## Rule IDs

- `ddl.table.comment.require`
- `ddl.table.name.max_length`
- `ddl.table.primary_key.require`
- `ddl.table.primary_key.columns.max_count`
- `ddl.table.columns.min_count`
- `ddl.table.audit_columns.require`
- `ddl.column.comment.require`
- `ddl.column.name.max_length`
- `ddl.column.varchar.max_length`
- `ddl.column.default.require`
- `ddl.column.not_null.require`
- `ddl.column.float_double.forbid`
- `ddl.index.total.max_count`
- `ddl.index.columns.max_count`
- `ddl.index.unique.prefix.require`
- `ddl.index.secondary.prefix.require`
- `ddl.index.fulltext.prefix.require`
- `ddl.index.duplicate.forbid`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
