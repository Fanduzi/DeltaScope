# DeltaScope Source-Aware Alter Checks Design

## Goal

Deepen `ALTER TABLE` auditing from coarse offline shape checks into source-aware, relation-aware alter judgment. The milestone should keep the current parser-neutral model, but add enough semantic structure to decide whether a change is plausibly safe, risky, or clearly incompatible without live database access.

## Current State

Milestone 2 delivered:

- richer parser-neutral `spec.Alter`
- richer alter extraction for column/index/rename/option changes
- rename-index forbids
- target-type-family allowlists for `MODIFY COLUMN` and `CHANGE COLUMN`
- alter-added index prefix checks

That is useful, but still shallow in the highest-value places:

- no source-to-target comparison for column type changes
- no semantic handling for nullability/default/unsigned transitions
- no stronger alter-index lifecycle governance beyond prefix naming

## Recommended Direction

### Option A: keep adding isolated alter rules against current payloads

Pros:
- fastest short-term progress

Cons:
- pushes increasingly complex logic into ad hoc rule code
- source-aware checks become string-heavy and fragile

### Option B: enrich alter extraction with explicit source/target change facts

Pros:
- keeps parser knowledge in the application layer
- gives rules a stable semantic substrate
- lets later online/schema-aware checks reuse the same model

Cons:
- requires extractor and domain evolution first

### Option C: defer source-aware checks until live schema support exists

Pros:
- avoids partial offline semantics

Cons:
- leaves the biggest DDL gap open too long
- blocks meaningful progress on alter governance

## Decision

Choose **Option B**.

Add a second semantic layer to the existing alter model: not live-schema truth, but normalized change facts that compare what the statement says it is changing. The rule layer should be able to reason about:

- source column identity
- target column identity
- source and target type family
- source and target nullability/default/unsigned/autoincrement flags
- index add/drop/rename lifecycle details

## Proposed Model Evolution

Keep `DDL.Alter []Alter` as the single alter stream, but enrich typed alter payloads with relation-aware detail:

- `AlterColumn`
  - source identity
  - target definition
  - source definition subset when statically present in the statement shape
  - normalized change flags such as rename / type change / nullability change / default change
- `AlterIndex`
  - lifecycle action kind
  - old/new names
  - added target definition

The extractor should not invent live schema facts. It should only preserve what can be inferred from the statement itself.

## Rule Surface For This Milestone

### Column-change rules

- source-to-target type compatibility policy
- nullability tightening/loosening policy
- default-change policy
- unsigned and auto-increment transition policy
- rename-plus-type-change compound restrictions

### Alter-index rules

- added index width / column-count checks
- duplicate-index checks for alter-added indexes
- drop/rename index governance beyond pure forbid toggles

## Non-Goals

- online object existence checks
- actual row-count / impact estimation
- complete create-table superset work
- HTTP API or MCP service work

## Expected Outcome

After this milestone, `ALTER TABLE` should move from “better normalized” to “meaningfully judged.” The remaining biggest DDL gap should then shift back to `CREATE TABLE` breadth, making the next milestone a clean create-table superset push.
