# Domain DML Rule Module

First Tier-1 DML rule batch for offline update/delete/insert checks.

## Files

| File | Responsibility |
|------|---------------|
| common.go | Shared DML rule IDs and operation predicates |
| config.go | Parses policy params for DML rule constructors |
| denylist_rules.go | Implements DML table denylist checks against protected schemas or tables |
| mutation_rules.go | Implements WHERE, LIMIT, ORDER BY, subquery, and JOIN ... ON rules |
| insert_rules.go | Implements insert row-count, replace, insert-select, and on-duplicate rules |
| register.go | Registers enabled DML rules into the shared registry |
| mutation_rules_test.go | Verifies update/delete rule behavior |
| insert_rules_test.go | Verifies insert-family rule behavior |
| register_test.go | Verifies policy-backed DML rule registration and deterministic ordering |

## Exports

- `Register(registry *rule.Registry, cfg policy.Policy) error`

## Dependencies
- Upstream: future application rule assembly and higher-level audit services
- Downstream: `internal/domain/policy`, `internal/domain/rule`, `internal/domain/spec`

## Rule IDs

- `dml.where.require`
- `dml.limit.forbid`
- `dml.order_by.forbid`
- `dml.subquery.forbid`
- `dml.join.on.require`
- `dml.insert.rows.max_count`
- `dml.replace.forbid`
- `dml.insert.select.forbid`
- `dml.insert.on_duplicate.forbid`
- `dml.table.denylist.forbid`

## Object-Scope Denylist Surface

- `dml.table.denylist.forbid`
- matches by `schemas`, `tables`, or `qualified_tables`
- evaluates every extracted DML target table from the parser-neutral spec
- stays inert by default because the shipped policy keeps those lists empty

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
