# DeltaScope DDL Alter Batch Design

## Goal

Close the next major offline DDL gap by adding safe `ALTER TABLE` restriction rules that operate on normalized alter actions without requiring live database metadata.

## Context

The current extractor already normalizes alter actions into simple domain values:

- `add_columns`
- `drop_column`
- `modify_column`
- `change_column`
- `rename_column`
- `rename_table`
- `drop_primary_key`
- `drop_index`
- `add_constraint`
- `table_option`

This is not enough for full alter analysis, but it is enough for action-level forbid rules.

## Approaches

### Option A: wait for a richer alter model first

Pros:
- avoids future migration of rule inputs

Cons:
- leaves a large DDL risk surface uncovered
- slows progress on an area where some offline rules are already safe

### Option B: add action-level forbid rules now, keep them coarse

Implement policy-driven restrictions for the alter actions that are already normalized:

- drop column
- drop primary key
- drop index
- rename table
- rename column
- change column
- modify column

Pros:
- high value immediately
- no parser leakage into rules
- matches several `gAudit` governance switches

Cons:
- still too coarse for type-compatibility or existence checks

### Option C: try to solve alter semantics deeply in one batch

Pros:
- stronger long-term model

Cons:
- much larger modeling effort
- likely to produce rushed abstractions while the repo is still growing quickly

## Decision

Choose **Option B**.

DeltaScope should capture the high-signal offline alter restrictions now, while preserving space for a later richer alter model.

## Rule Batch

Planned rule IDs:

- `ddl.alter.drop_column.forbid`
- `ddl.alter.drop_primary_key.forbid`
- `ddl.alter.drop_index.forbid`
- `ddl.alter.rename_table.forbid`
- `ddl.alter.rename_column.forbid`
- `ddl.alter.change_column.forbid`
- `ddl.alter.modify_column.forbid`

## Policy Defaults

Recommended defaults:

- drop column: allowed by default
- drop primary key: forbidden by default
- drop index: allowed by default
- rename table: forbidden by default
- rename column: forbidden by default
- change column: forbidden by default
- modify column: allowed by default

This mirrors the current product stance: block the riskiest structural rewrites by default, but avoid forbidding every alter operation globally.

## Deferred Work

- column-type compatibility analysis
- existence checks for dropped/renamed objects
- index rename restrictions
- merge-alter-table restrictions for MySQL/TiDB
- object/table-option validation beyond create-table

## Verification

- extend alter extraction assertions only if needed
- add focused rule tests for each forbidden action
- keep registry ordering coverage deterministic
- re-run:
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
