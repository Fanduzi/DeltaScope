# DDL Table-Option Batch Implementation Plan

**Goal:** add the next create-table DDL slice for engine, charset, comment length, and object-shape restrictions.

**Architecture:** enrich the create-table domain shape with a few parser-neutral booleans, then keep rule logic inside `internal/domain/rule/ddl`.

**Tech Stack:** Go, TiDB parser AST extraction, Go testing

---

### Task 1: Enrich create-table shape extraction

**Files:**
- Modify: `internal/domain/spec/ddl.go`
- Modify: `internal/domain/spec/README.md`
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Modify: `internal/application/audit/README.md`

**Acceptance:**
- create-table extraction captures:
  - refer-table / create-like
  - select-backed create-table / create-as
  - partition presence
- tests cover the new booleans

### Task 2: Add table-option/object-shape rules

**Files:**
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`
- Modify: `internal/domain/policy/README.md`
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/rule/ddl/register_test.go`
- Modify: `internal/domain/rule/ddl/README.md`
- Create: `internal/domain/rule/ddl/table_option_rules.go`
- Create: `internal/domain/rule/ddl/table_option_rules_test.go`

**Acceptance:**
- add rule IDs:
  - `ddl.table.comment.max_length`
  - `ddl.table.engine.allowlist`
  - `ddl.table.charset.allowlist`
  - `ddl.table.foreign_key.forbid`
  - `ddl.table.partition.forbid`
  - `ddl.table.create_like.forbid`
  - `ddl.table.create_as.forbid`
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
- README and handoff reflect the new create-table option coverage
