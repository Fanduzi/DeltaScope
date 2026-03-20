# DeltaScope DDL Primary-Key Semantics Design

## Goal

Close another high-value `gAudit` gap by enforcing stronger primary-key semantics for the common single-column case.

## Context

DeltaScope already checks:

- primary key presence
- primary key column-count limit

It does not yet enforce the stronger shape conventions that `gAudit` expects for many teams:

- bigint primary key
- unsigned primary key
- auto-increment primary key
- explicit not-null primary key

## Approaches

### Option A: keep primary-key rules structural only

Pros:
- no extractor change

Cons:
- misses some of the most opinionated and useful governance checks

### Option B: enrich column metadata slightly and add semantic PK rules

Add:

- `Column.Unsigned`
- `Column.AutoIncrement`

Then evaluate the declared primary-key columns against those fields.

Pros:
- small model change
- strong value for common single-column PK patterns
- still fully offline and parser-neutral in the rule layer

Cons:
- composite primary keys still need separate treatment

### Option C: wait for identifier and type-family work first

Pros:
- fewer concurrent DDL directions

Cons:
- delays another clear `gAudit` parity win

## Decision

Choose **Option B**.

This batch is small, self-contained, and captures rules that many teams actually care about in review.

## Model Changes

Extend `spec.Column` with:

- `Unsigned bool`
- `AutoIncrement bool`

Rules will look up the primary-key columns by name from the normalized column list.

## Rule Batch

Planned rule IDs:

- `ddl.table.primary_key.bigint.require`
- `ddl.table.primary_key.unsigned.require`
- `ddl.table.primary_key.auto_increment.require`
- `ddl.table.primary_key.not_null.require`

## Policy Defaults

Recommended defaults:

- bigint required: true
- unsigned required: true
- auto increment required: true
- not null required: true

These defaults match the stricter convention already implied by the existing project direction.

## Deferred Work

- smarter handling for composite PK semantics
- key-name identifier checks
- auto-increment initial-value checks

## Verification

- extend extraction tests for unsigned/auto_increment metadata
- add focused PK-semantic rule tests
- re-run:
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
