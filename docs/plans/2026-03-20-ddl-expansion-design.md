# DeltaScope DDL Expansion Design

## Goal

Extend `DeltaScope` beyond the first `CREATE TABLE` batch so the DDL rule catalog moves closer to the long-term `gAudit` superset goal without breaking the current offline-only architecture.

## Scope

This expansion is split into three batches:

1. column and audit-column rules
2. index rules
3. alter restrictions

The first implementation wave will start with batch 1 because the current extracted `spec.DDL` shape already contains enough stable column information to support it.

## Recommended Approach

### Option A: keep rules shallow and avoid model changes

Add only rules that can be inferred from existing `spec.Column{Name, Type, Comment}`.

Pros:
- fastest
- low-risk

Cons:
- audit-column checks stay too weak
- column default and nullability rules remain impossible
- would force a second extractor rewrite soon

### Option B: extend the column model once, then add a coherent column-rule batch

Extend `spec.Column` and the extractor with the minimum extra semantics needed for column rules:

- length
- not-null flag
- default presence/value
- `CURRENT_TIMESTAMP` default
- `ON UPDATE CURRENT_TIMESTAMP`

Then add a focused rule batch for:

- table must contain columns
- audit timestamp columns
- column comment requirement
- column name max length
- varchar max length
- default-value requirement
- not-null requirement with type allowlist
- float/double guidance

Pros:
- highest leverage per extractor change
- aligns with `gAudit` column rules
- avoids parser-AST leakage into rules

Cons:
- slightly larger first batch

### Option C: jump straight to index + alter restrictions

Pros:
- attacks larger remaining DDL gaps sooner

Cons:
- current `spec.Index` and `spec.Alter` shapes are still too thin
- likely to produce brittle rules or another rushed refactor

## Decision

Choose **Option B**.

`DeltaScope` should first become strong on create-table column semantics before moving into index naming/shape and alter-action governance. This keeps the next batch valuable, testable, and structurally clean.

## Model Changes

`spec.Column` should grow only where the upcoming rules need real semantics:

- `Length int`
- `NotNull bool`
- `HasDefault bool`
- `DefaultValue string`
- `DefaultIsNull bool`
- `DefaultIsCurrentTimestamp bool`
- `OnUpdateCurrentTimestamp bool`

Rules still consume `spec.Statement`; parser details remain hidden in the application extractor.

## Rule Batch 2

Planned rule IDs:

- `ddl.table.columns.min_count`
- `ddl.table.audit_columns.require`
- `ddl.column.comment.require`
- `ddl.column.name.max_length`
- `ddl.column.varchar.max_length`
- `ddl.column.default.require`
- `ddl.column.not_null.require`
- `ddl.column.float_double.forbid`

Notes:

- audit-column validation stays name-agnostic, matching `gAudit`'s documented behavior
- `not_null.require` should allow nullable blob/text/json columns and optionally time-like columns through policy
- default-value checks stay structural, not database-runtime aware

## Deferred Batches

### Index batch

Needs richer `spec.Index` metadata such as uniqueness/fulltext classification before rules like prefix requirements can be cleanly implemented.

### Alter batch

Needs richer `spec.Alter` detail than the current `Action + Name` pair before drop/rename/type-change restrictions can be modeled safely.

## Testing Strategy

- extend extraction tests first
- add focused rule tests per concern
- keep registry integration coverage deterministic
- re-run `go test ./internal/application/audit/...`, `go test ./internal/domain/rule/ddl/...`, then `go test ./...`

## Documentation

Update:

- decision log
- `internal/domain/spec/README.md`
- `internal/application/audit/README.md`
- `internal/domain/rule/ddl/README.md`
- root `README.md` only if module boundaries change
