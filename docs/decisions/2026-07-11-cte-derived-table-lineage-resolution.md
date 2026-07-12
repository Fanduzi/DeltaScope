# CTE and Derived Table Lineage Resolution

- Status: accepted
- Date: 2026-07-11
- Related milestone/version: v0.380.0

## Context

Query access analysis for CTEs and derived tables previously emitted virtual column sources (e.g., `cte.id`, `x.id`) instead of tracing through to the underlying physical table columns (e.g., `users.id`). This made downstream permission checks inaccurate: CTE/derived table names appeared as permission-bearing objects when they should not be.

Additionally, derived tables (`kind: "derived"`) were incorrectly marked as `PermissionRequired: true` because the permission logic only excluded CTEs (`kind != "cte"`), not derived tables.

## Decision

### Lineage Map

Build a lineage map during CTE and derived table analysis that records which physical source columns each virtual output column maps to. When the outer query references a CTE/derived table column, resolve through the lineage map to find the physical sources.

- TiDB parser: `lineageMap` type in `scopeStack`, populated from CTE body outputs and derived table subquery outputs. `columnCollectVisitor.Enter` checks lineage before default resolution.
- PostgreSQL parser: `cteLineageMap` type, built recursively from CTE bodies. `collectOutputs` and `collectRefsFromNode` resolve through lineage when the table qualifier matches a CTE name.

### Permission Semantics

`PermissionRequired` is now `true` only for `table` and `view` kinds. CTEs and derived tables are `PermissionRequired: false`.

### Scope Inheritance

CTE scopes inherit previously defined CTEs' lineage from the parent scope, enabling nested CTE resolution (`WITH a AS (...), b AS (SELECT * FROM a) SELECT * FROM b`).

## Public Contract

- `outputs[].sources` now contains physical source keys (e.g., `users.id`) instead of virtual names (e.g., `cte.id`).
- `referenced_columns` now shows physical table references for CTE/derived column accesses (merged with CTE body references).
- `relations` entries for CTEs and derived tables have `permission_required: false`.
- `requirements` no longer include `read_table` or `read_column` for CTE/derived names.

## Verification

- `go test ./internal/infrastructure/parser/tidb/ -run QueryAccess -count=1`
- `go test ./internal/application/queryaccess/ -count=1`
- `go test -race ./internal/application/queryaccess/ -count=1`
- `go vet ./internal/infrastructure/parser/tidb/ ./internal/application/queryaccess/`

## Deferred

- PostgreSQL derived table lineage (not explicitly tested in corpus; CTE lineage implemented).
- Wildcard expansion through CTE/derived lineage.

## Release-Surface Evidence (v0.380.0)

- Version assigned: `v0.380.0` with the Query Access Analysis Foundation release.
- Public contract restated in release notes: CTE/derived tables are not permission objects; lineage resolves to physical table/view sources for requirements and outputs.
- No production behavior change in this release-surface commit; only version pins, docs, and decision-record version assignment.
- Tag/push/publish deferred to a later readiness audit.
