# DDL Primary-Key Semantics Implementation Plan

**Goal:** add primary-key semantic rules for bigint, unsigned, auto-increment, and not-null requirements.

**Architecture:** enrich normalized column metadata slightly, then keep semantic rule evaluation inside `internal/domain/rule/ddl`.

**Tech Stack:** Go, TiDB parser AST extraction, Go testing

---

### Task 1: Enrich extracted column metadata

**Files:**
- Modify: `internal/domain/spec/ddl.go`
- Modify: `internal/domain/spec/README.md`
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Modify: `internal/application/audit/README.md`

**Acceptance:**
- column extraction captures `Unsigned` and `AutoIncrement`
- extraction tests cover both flags

### Task 2: Add primary-key semantic rules

**Files:**
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`
- Modify: `internal/domain/policy/README.md`
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/rule/ddl/register_test.go`
- Modify: `internal/domain/rule/ddl/README.md`
- Create: `internal/domain/rule/ddl/primary_key_semantic_rules.go`
- Create: `internal/domain/rule/ddl/primary_key_semantic_rules_test.go`

**Acceptance:**
- add rule IDs:
  - `ddl.table.primary_key.bigint.require`
  - `ddl.table.primary_key.unsigned.require`
  - `ddl.table.primary_key.auto_increment.require`
  - `ddl.table.primary_key.not_null.require`
- defaults and example config stay aligned with `config init`
- registry integration covers the new rules

### Task 3: Verify and record batch progress

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`

**Acceptance:**
- full test suite passes
- three-level-doc check passes
- README and handoff reflect stronger PK semantics
