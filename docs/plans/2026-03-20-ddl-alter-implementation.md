# DDL Alter Batch Implementation Plan

**Goal:** add the next offline-safe `ALTER TABLE` rule slice by enforcing configurable action-level restrictions.

**Architecture:** reuse the existing normalized `spec.Alter` actions, keep rules inside `internal/domain/rule/ddl`, and avoid new parser leakage or online metadata dependencies.

**Tech Stack:** Go, TiDB parser AST extraction, Go testing

---

### Task 1: Add alter restriction rules

**Files:**
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`
- Modify: `internal/domain/policy/README.md`
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/rule/ddl/register_test.go`
- Modify: `internal/domain/rule/ddl/README.md`
- Create: `internal/domain/rule/ddl/alter_rules.go`
- Create: `internal/domain/rule/ddl/alter_rules_test.go`

**Acceptance:**
- add rule IDs:
  - `ddl.alter.drop_column.forbid`
  - `ddl.alter.drop_primary_key.forbid`
  - `ddl.alter.drop_index.forbid`
  - `ddl.alter.rename_table.forbid`
  - `ddl.alter.rename_column.forbid`
  - `ddl.alter.change_column.forbid`
  - `ddl.alter.modify_column.forbid`
- defaults and example config stay aligned with `config init`
- deterministic registration test includes alter rules

### Task 2: Verify and record batch progress

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`

**Acceptance:**
- full test suite passes
- three-level-doc check passes
- root README mentions the new alter coverage
- decision log records why action-level alter rules land before richer alter modeling
