# Domain DML Rule Module

First Tier-1 DML rule batch for offline update/delete/insert checks.

## Files

| File | Responsibility |
|------|---------------|
| common.go | Shared DML rule IDs and operation predicates |
| config.go | Parses policy params for DML rule constructors |
| denylist_rules.go | Implements DML table denylist checks against protected schemas or tables |
| metadata_rules.go | Implements the MySQL/TiDB metadata-backed DML target-table existence blocker |
| mutation_rules.go | Implements WHERE, LIMIT, ORDER BY, subquery, and JOIN ... ON rules |
| impact_rules.go | Implements additive statement-level impact estimation plus row-count / ratio thresholds |
| insert_rules.go | Implements insert row-count, replace, insert-select, and on-duplicate rules |
| register.go | Registers enabled DML rules into the shared registry |
| mutation_rules_test.go | Verifies update/delete rule behavior |
| impact_rules_test.go | Verifies impact estimate threshold and registration behavior |
| insert_rules_test.go | Verifies insert-family rule behavior |
| register_test.go | Verifies policy-backed DML rule registration and deterministic ordering |
| metadata_rules_test.go | Verifies metadata-backed target-table existence and dialect boundaries |

## Exports

- `Register(registry *rule.Registry, cfg policy.Policy) error`

## Dependencies
- Upstream: future application rule assembly and higher-level audit services
- Downstream: `internal/domain/policy`, `internal/domain/rule`, `internal/domain/spec`

## Rule IDs

- `dml.where.require`
- `dml.impact.estimate`
- `dml.impact.rows.max_count`
- `dml.impact.ratio.max_percent`
- `dml.limit.forbid`
- `dml.order_by.forbid`
- `dml.subquery.forbid`
- `dml.join.on.require`
- `dml.insert.rows.max_count`
- `dml.replace.forbid`
- `dml.insert.select.forbid`
- `dml.insert.on_duplicate.forbid`
- `dml.table.denylist.forbid`
- `dml.table.exists.require`

`dml.table.exists.require` evaluates a single resolved mutation target. Ambiguous multi-target UPDATE/DELETE statements fail closed until the statement model carries per-target snapshots.

`dml.impact.estimate`, `dml.impact.rows.max_count`, and `dml.impact.ratio.max_percent` are cataloged as default-disabled. Default Policy does not enable them; caller config must opt in before they emit findings.

## Object-Scope Denylist Surface

- `dml.table.denylist.forbid`
- matches by `schemas`, `tables`, or `qualified_tables`
- evaluates every extracted DML target table from the parser-neutral spec
- stays inert by default because the shipped policy keeps those lists empty

## Impact Output Surface

- the audit flow attaches a conservative statement-level `impact` object for `UPDATE` and `DELETE` before rule evaluation, including default audits that do not load the impact rules
- `dml.impact.estimate` reports that precomputed estimate as rule output when enabled; it does not control whether the payload exists
- the additive payload includes `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`
- offline mode uses SQL shape only
- metadata-aware mode may refine the estimate with read-only table statistics
- `dml.impact.rows.max_count` and `dml.impact.ratio.max_percent` evaluate that shared `impact` payload without changing existing rule inputs

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
