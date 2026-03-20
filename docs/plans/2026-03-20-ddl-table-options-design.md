# DeltaScope DDL Table-Option Batch Design

## Goal

Push DeltaScope's offline DDL coverage deeper into `gAudit`'s table-level governance surface by adding rules for create-table options and object-shape controls.

## Context

Current extraction already preserves:

- table comment
- engine
- charset
- non-index constraints

The remaining offline-safe table-level gaps are:

- comment length
- engine allowlist
- charset allowlist
- foreign-key restriction
- partition restriction
- `CREATE TABLE ... LIKE`
- `CREATE TABLE ... AS SELECT`

## Approaches

### Option A: keep extraction unchanged and only use existing `Options`/`Constraints`

Pros:
- smallest code change

Cons:
- cannot distinguish `CREATE TABLE ... LIKE`, `... AS SELECT`, or partitioned tables
- leaves several high-value `gAudit`-style switches unavailable

### Option B: minimally enrich create-table shape and add a coherent option batch

Add only a few booleans to the domain DDL model:

- `HasReferTable`
- `HasSelect`
- `HasPartition`

Then implement table-level rules for comment length, engine, charset, foreign key, partition, create-like, and create-as.

Pros:
- strong offline coverage with small model growth
- still parser-neutral
- cleanly maps to policy and future output

Cons:
- slightly broader than a pure option-only batch

### Option C: wait until after richer primary-key/type modeling

Pros:
- fewer concurrent DDL directions

Cons:
- delays a large set of easy, high-value offline rules

## Decision

Choose **Option B**.

This batch adds real coverage without forcing deep new abstractions. The extra DDL booleans are stable enough to justify now.

## Model Changes

Extend `spec.DDL` with:

- `HasReferTable bool`
- `HasSelect bool`
- `HasPartition bool`

These stay narrow and specific to create-table shape.

## Rule Batch

Planned rule IDs:

- `ddl.table.comment.max_length`
- `ddl.table.engine.allowlist`
- `ddl.table.charset.allowlist`
- `ddl.table.foreign_key.forbid`
- `ddl.table.partition.forbid`
- `ddl.table.create_like.forbid`
- `ddl.table.create_as.forbid`

## Policy Defaults

Recommended defaults:

- table comment max length: `128`
- engine allowlist: `["InnoDB"]`
- charset allowlist: `["utf8", "utf8mb4"]`
- foreign key: forbidden
- partition: forbidden
- create like: forbidden
- create as: forbidden

## Deferred Work

- identifier/keyword validation
- column charset/collation validation
- table charset recommendation vs strict allowlist mapping
- auto-increment initial value validation
- create view/drop/truncate/drop-table governance

## Verification

- extend extraction tests first
- add focused rule tests per option/object concern
- re-run:
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
