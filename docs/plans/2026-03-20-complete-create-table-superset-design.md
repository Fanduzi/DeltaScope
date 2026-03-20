# DeltaScope Complete Create-Table Superset Design

## Goal

Finish the offline `CREATE TABLE` rule surface so it clearly surpasses `gAudit` on breadth, structure, and usability.

## Current State

`DeltaScope` already covers a strong core:

- table comment/name/basic option checks
- primary-key presence and semantics
- audit columns
- column comment/default/not-null/float-double/varchar rules
- create-table index count/prefix/duplicate checks
- object-shape forbids such as foreign key, partitioning, `LIKE`, and `AS SELECT`

The main remaining create-table gaps are breadth-focused rather than architectural:

- identifier and keyword checks
- deeper type-family allow/forbid rules
- charset/collation guidance
- stronger redundant-index analysis
- a few remaining table-option and object-shape specifics

## Recommended Direction

Treat this milestone as a deliberate breadth push. Avoid inventing new big abstractions unless they pay off across multiple rule families immediately. Reuse the existing create-table parser-neutral extraction and DDL rule framework.

## Rule Families To Finish

- identifier legality and reserved-keyword governance
- broader type-family policy:
  - blob/json/bit/timestamp family decisions
  - `char` vs `varchar` guidance
- column charset/collation restrictions where statically available
- deeper redundant-index analysis beyond exact duplicates
- remaining table-option and object-shape checks worth doing offline

## Non-Goals

- live schema existence checks
- HTTP API or MCP work
- broad redesign of the current create-table rule framework

## Expected Outcome

After this milestone, `CREATE TABLE` should no longer be “strong but incomplete.” It should be a clear offline create-table superset relative to `gAudit`, leaving the next product-level milestone free to shift toward service delivery or remaining cross-cutting DDL cleanup.
