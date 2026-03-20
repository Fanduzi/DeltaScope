# DDL Index Batch Implementation Plan

**Goal:** add the next offline-safe `CREATE TABLE` DDL rule slice by enriching index metadata and implementing index-governance rules.

**Architecture:** extend the domain index model just enough to classify index kind, enrich application extraction, then keep all rule logic inside `internal/domain/rule/ddl`.

**Tech Stack:** Go, TiDB parser AST extraction, Go testing

---

### Task 1: Enrich extracted index metadata

**Files:**
- Modify: `internal/domain/spec/ddl.go`
- Modify: `internal/domain/spec/README.md`
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Modify: `internal/application/audit/README.md`

**Acceptance:**
- `spec.Index` carries a typed `Kind`
- extraction maps create-table constraints into stable index kinds
- extraction tests cover secondary, unique, and fulltext index shapes

### Task 2: Add index-governance rules

**Files:**
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`
- Modify: `internal/domain/policy/README.md`
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/rule/ddl/register_test.go`
- Modify: `internal/domain/rule/ddl/README.md`
- Create: `internal/domain/rule/ddl/index_rules.go`
- Create: `internal/domain/rule/ddl/index_rules_test.go`

**Acceptance:**
- add rule IDs:
  - `ddl.index.total.max_count`
  - `ddl.index.columns.max_count`
  - `ddl.index.unique.prefix.require`
  - `ddl.index.secondary.prefix.require`
  - `ddl.index.fulltext.prefix.require`
  - `ddl.index.duplicate.forbid`
- defaults and example config stay aligned with `config init`
- deterministic registration test includes the new index rules

### Task 3: Verify and record batch progress

**Files:**
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`

**Acceptance:**
- full test suite passes
- three-level-doc check passes
- decision log records why alter restrictions remain deferred
- overnight handoff reflects completed index batch and remaining DDL gaps
