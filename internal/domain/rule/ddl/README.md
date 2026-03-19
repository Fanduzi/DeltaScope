# Domain DDL Rule Module

First Tier-1 DDL rule batch for create-table checks.

## Files

| File | Responsibility |
|------|---------------|
| common.go | Shared DDL rule IDs and create-table applicability checks |
| config.go | Parses policy params for DDL rule constructors |
| table_rules.go | Implements table comment and table name rules |
| primary_key_rules.go | Implements primary-key presence and column-count rules |
| register.go | Registers enabled DDL rules into the shared registry |
| table_rules_test.go | Verifies table comment and name-length rule behavior |
| primary_key_rules_test.go | Verifies primary-key requirement and shape rules |
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

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
