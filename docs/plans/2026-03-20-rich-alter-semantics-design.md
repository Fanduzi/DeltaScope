# DeltaScope Rich Alter Semantics Design

## Goal

Make `ALTER TABLE` a first-class, parser-neutral audit target instead of treating it as a flat action list. The result should unlock a broader offline rule surface while preserving the existing DDD-leaning architecture.

## Current State

`DeltaScope` already supports action-level alter restrictions through normalized `spec.Alter{Action, Name}` records. That is enough for coarse forbids such as:

- `drop_column`
- `drop_primary_key`
- `drop_index`
- `rename_table`
- `rename_column`
- `change_column`
- `modify_column`

This is still too thin for the next high-value DDL gap:

- column type-change compatibility
- rename semantics beyond coarse forbids
- add/drop index details
- alter table option changes
- future merge-alter restrictions

## Recommended Direction

### Option A: keep `spec.Alter` flat and keep adding action-specific rules

Pros:
- fastest short term
- no new extractor work

Cons:
- turns `ALTER TABLE` into another string-switch subsystem
- weak foundation for future HTTP/MCP reuse
- hard to express richer semantics cleanly

### Option B: enrich `spec.Alter` with typed detail structs under one normalized shape

Represent each alter action as a normalized record with optional typed detail:

- column add/drop/modify/change/rename
- index add/drop/rename
- table option change

Pros:
- strongest long-term structure
- keeps rules parser-neutral
- preserves one unified `DDL.Alter` entrypoint

Cons:
- needs careful YAGNI discipline

### Option C: split alter semantics into many separate top-level slices on `spec.DDL`

Pros:
- simple for some direct consumers

Cons:
- model fragmentation
- harder to keep alter sequencing coherent
- grows `spec.DDL` into a grab bag

## Decision

Choose **Option B**.

Keep `DDL.Alter` as the single alter stream, but expand it into a richer normalized model with optional typed detail. That preserves a clean domain boundary and gives future rules a stable substrate.

## Proposed Domain Shape

Keep:

- `DDL.Alter []Alter`

Evolve `Alter` roughly into:

- `Action AlterAction`
- `Name string`
- `Column *AlterColumn`
- `Index *AlterIndex`
- `Options map[string]string`

Where:

- `AlterColumn` carries old/new names, normalized type, length, unsigned, not-null, auto-increment, default flags, and comment
- `AlterIndex` carries kind, name, old/new names, and columns

The model should only include fields that upcoming rules actually need.

## First Rich-Alter Rule Batch

Use the richer model to implement offline-safe rules for:

- `change column` forbid
- `modify column` forbid
- compatible vs incompatible type changes
- `rename column` forbid with explicit old/new names
- `rename index` forbid
- `drop index` / `drop column` restrictions with clearer semantics
- add-index prefix and width rules on alter-added indexes

This batch should still avoid live metadata assumptions such as:

- whether a dropped column already exists
- whether an index rename collides online

## Extraction Boundary

All TiDB AST inspection stays inside `internal/application/audit`.

The application extractor should:

1. parse raw alter specs
2. normalize them into richer domain `Alter` records
3. never expose TiDB AST to the domain rule layer

## Testing Strategy

- extend extraction tests first with representative alter cases
- add focused domain-rule tests per alter concern
- keep registry-order coverage explicit
- verify full-suite compatibility after each alter batch

## Out Of Scope For This Milestone

- online existence checks
- schema snapshot comparison
- merge-alter behavior that requires deeper dialect/version semantics
- every remaining create-table gap

## Expected Outcome

After this milestone, `ALTER TABLE` should no longer be a thin special case. It should have a stable domain model that supports meaningful offline governance and gives `DeltaScope` a stronger path to eventually surpass `gAudit` on DDL semantics overall.
