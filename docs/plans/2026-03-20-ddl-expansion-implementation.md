# DDL Expansion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add the second DDL rule batch for column and audit-column governance while preserving the current offline, parser-neutral rule architecture.

**Architecture:** Extend `spec.Column` with only the semantics required by the new rules, enrich extraction inside `internal/application/audit`, then add independent DDL rules that operate purely on `spec.Statement`.

**Tech Stack:** Go, TiDB parser AST extraction, Go testing

---

### Task 1: Extend column extraction semantics

**Files:**
- Modify: `internal/domain/spec/ddl.go`
- Modify: `internal/domain/spec/README.md`
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Modify: `internal/application/audit/README.md`

**Step 1: Write failing extractor assertions**

Add coverage for:
- varchar length extraction
- not-null extraction
- default presence/value extraction
- current-timestamp default extraction
- on-update current-timestamp extraction

**Step 2: Run targeted tests**

Run: `go test ./internal/application/audit -run TestExtract -v`
Expected: FAIL.

**Step 3: Implement minimal extraction**

Populate the new `spec.Column` fields from TiDB column options without leaking AST into the domain model.

**Step 4: Re-run tests**

Run: `go test ./internal/application/audit -run TestExtract -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: enrich ddl column extraction"
```

### Task 2: Add column and audit-column DDL rules

**Files:**
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`
- Modify: `internal/domain/policy/README.md`
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/rule/ddl/README.md`
- Create: `internal/domain/rule/ddl/column_rules.go`
- Create: `internal/domain/rule/ddl/column_rules_test.go`
- Create: `internal/domain/rule/ddl/audit_column_rules.go`
- Create: `internal/domain/rule/ddl/audit_column_rules_test.go`
- Modify: `internal/domain/rule/ddl/register_test.go`

**Step 1: Write failing rule tests**

Cover:
- empty column list
- missing audit timestamp pair
- missing column comment
- column name too long
- varchar length too large
- missing default value
- nullable non-allowed column
- float/double column

**Step 2: Run targeted tests**

Run: `go test ./internal/domain/rule/ddl/... -v`
Expected: FAIL.

**Step 3: Implement the rule batch**

Keep one concern group per file and stable `rule_id`s.

**Step 4: Wire policy defaults and example config**

Add default levels and params for the new rules.

**Step 5: Re-run tests**

Run: `go test ./internal/domain/rule/ddl/... -v`
Expected: PASS.

**Step 6: Commit**

```bash
git add .
git commit -m "feat: add ddl column governance rules"
```

### Task 3: Verify and record the new DDL batch

**Files:**
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`

**Step 1: Run full verification**

Run: `go test ./...`
Expected: PASS.

**Step 2: Update decision and handoff docs**

Record:
- what the new DDL batch covers
- what remains intentionally deferred

**Step 3: Commit**

```bash
git add .
git commit -m "docs: record second ddl batch progress"
```
