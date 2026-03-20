# DeltaScope DDL Index Batch Design

## Goal

Extend `DeltaScope`'s offline DDL coverage with a focused `CREATE TABLE` index-governance batch that moves closer to `gAudit`'s index rule surface without forcing premature `ALTER TABLE` modeling.

## Context

The second DDL batch already enriched column semantics and added column/audit-column governance. The next safest gap is index governance because the extracted `CREATE TABLE` shape already carries index names and indexed columns. The only missing semantic is index kind.

## Approaches

### Option A: keep `spec.Index` shallow and add only total-count checks

Pros:
- minimal changes
- very low implementation risk

Cons:
- cannot express unique/fulltext prefix rules
- leaves `gAudit`'s index configuration mostly uncovered

### Option B: add index kind once and implement a coherent create-table index batch

Extend `spec.Index` with a typed `Kind` and map TiDB constraint kinds into that model. Then implement:

- total secondary-index count
- per-index column-count limit
- unique-index prefix requirement
- secondary-index prefix requirement
- fulltext-index prefix requirement
- duplicate-index detection

Pros:
- high value with small model growth
- aligns with `gAudit` index policy surface
- remains parser-neutral and offline-safe

Cons:
- slightly broader than a pure count-only batch

### Option C: jump to alter restrictions now

Pros:
- addresses more dangerous DDL actions sooner

Cons:
- current `spec.Alter` only has `Action + Name`
- high chance of brittle rules and another forced refactor

## Decision

Choose **Option B**.

`DeltaScope` should finish a strong `CREATE TABLE` governance surface first. Extending `spec.Index` with `Kind` is a cheap, stable improvement that unlocks several meaningful rules immediately, while alter restrictions still need richer extracted metadata.

## Model Changes

Add a typed index-kind field:

- `spec.IndexKind`
- `spec.Index.Kind`

Planned values:

- `IndexKindPrimary`
- `IndexKindSecondary`
- `IndexKindUnique`
- `IndexKindFulltext`
- `IndexKindUnknown`

Extraction rules:

- primary key stays in `DDL.PrimaryKey`, but should carry `KindPrimary`
- `KEY` / `INDEX` map to `KindSecondary`
- `UNIQUE*` map to `KindUnique`
- `FULLTEXT` maps to `KindFulltext`

## Rule Batch

Planned rule IDs:

- `ddl.index.total.max_count`
- `ddl.index.columns.max_count`
- `ddl.index.unique.prefix.require`
- `ddl.index.secondary.prefix.require`
- `ddl.index.fulltext.prefix.require`
- `ddl.index.duplicate.forbid`

## Policy Defaults

Recommended defaults:

- total index limit: `12`
- per-index column limit: `8`
- unique prefix: `uniq_`
- secondary prefix: `idx_`
- fulltext prefix: `full_`
- duplicate detection: enabled

The lowercase defaults fit DeltaScope's naming style better than `gAudit`'s uppercase examples, while remaining semantically equivalent.

## Deferred Work

- alter-table index rename/drop restrictions
- redundant-prefix detection beyond exact duplicates
- index expression / prefix-length aware duplicate analysis

## Verification

- extend extraction coverage first
- add focused rule tests per index concern
- keep registry ordering coverage deterministic
- re-run:
  - `go test ./internal/application/audit -run TestExtract -v`
  - `go test ./internal/domain/rule/ddl/... -v`
  - `go test ./...`
  - `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
  - `check_three_level_doc.sh`
