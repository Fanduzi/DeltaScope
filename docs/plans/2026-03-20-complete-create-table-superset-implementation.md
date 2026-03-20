# Complete Create-Table Superset Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** complete the offline `CREATE TABLE` rule surface so it clearly exceeds `gAudit` in create-table coverage.

**Architecture:** keep create-table extraction in `internal/application/audit`, expand only the parser-neutral facts already needed by upcoming rules, and add the remaining breadth-focused rule families inside `internal/domain/rule/ddl`.

**Tech Stack:** Go, TiDB parser AST, Cobra/Viper config flow, Go testing

---

### Task 1: Audit the remaining create-table gap and pin exact rule IDs

**Files:**
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/README.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`

**Step 1: Add the remaining create-table rule IDs**

Pin stable IDs for the chosen gap-closing rules.

**Step 2: Add/update docs**

Document which create-table families are now in scope for this milestone.

**Step 3: Commit**

```bash
git add internal/domain/rule/ddl/common.go internal/domain/rule/ddl/README.md docs/plans/2026-03-20-deltascope-v1-decisions.md
git commit -m "docs: pin create-table superset rule surface"
```

### Task 2: Add identifier and keyword governance

**Files:**
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Create: `internal/domain/rule/ddl/identifier_rules.go`
- Create: `internal/domain/rule/ddl/identifier_rules_test.go`
- Modify: `internal/domain/rule/ddl/register.go`

**Step 1: Write failing tests**

Cover:
- invalid identifier characters
- reserved keyword names
- table/column/index naming edge cases

**Step 2: Run targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'Test.*Identifier.*|Test.*Keyword.*' -v`
Expected: FAIL.

**Step 3: Implement minimal extraction/rules**

Preserve only facts needed for identifier checks.

**Step 4: Re-run tests**

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/domain/rule/ddl/identifier_rules.go internal/domain/rule/ddl/identifier_rules_test.go internal/domain/rule/ddl/register.go
git commit -m "feat: add create-table identifier governance"
```

### Task 3: Add wider type-family and charset/collation rules

**Files:**
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Create: `internal/domain/rule/ddl/type_family_rules.go`
- Create: `internal/domain/rule/ddl/type_family_rules_test.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/policy/defaults.go`
- Modify: `internal/domain/policy/README.md`
- Modify: `configs/deltascope.example.yaml`

**Step 1: Write failing tests**

Cover:
- blob/json/bit/timestamp family policy
- char-vs-varchar guidance
- charset/collation restrictions where available

**Step 2: Run targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'Test.*(Type|Charset|Collation).*' -v`
Expected: FAIL.

**Step 3: Implement minimal rules**

Keep semantics honest and offline-safe.

**Step 4: Align policy/config**

Update defaults and example config.

**Step 5: Re-run tests**

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/domain/rule/ddl/type_family_rules.go internal/domain/rule/ddl/type_family_rules_test.go internal/domain/rule/ddl/register.go internal/domain/policy/defaults.go internal/domain/policy/README.md configs/deltascope.example.yaml
git commit -m "feat: add create-table type-family governance"
```

### Task 4: Deepen redundant-index analysis

**Files:**
- Modify: `internal/domain/rule/ddl/index_rules.go`
- Modify: `internal/domain/rule/ddl/index_rules_test.go`

**Step 1: Write failing tests**

Cover:
- exact duplicate indexes
- left-prefix redundant indexes
- unique vs secondary overlap cases worth flagging offline

**Step 2: Run targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'Test.*Index.*' -v`
Expected: FAIL.

**Step 3: Implement minimal logic**

Extend existing index governance instead of replacing it.

**Step 4: Re-run tests**

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/rule/ddl/index_rules.go internal/domain/rule/ddl/index_rules_test.go
git commit -m "feat: deepen create-table redundant index checks"
```

### Task 5: Close remaining create-table object-shape gaps

**Files:**
- Modify: `internal/domain/rule/ddl/table_option_rules.go`
- Modify: `internal/domain/rule/ddl/table_option_rules_test.go`
- Modify: `internal/domain/policy/defaults.go`
- Modify: `configs/deltascope.example.yaml`

**Step 1: Write failing tests**

Cover the remaining chosen object-shape/table-option rules.

**Step 2: Run targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'Test.*(TableOption|ObjectShape).*' -v`
Expected: FAIL.

**Step 3: Implement minimal logic**

Keep the rule set focused on offline-safe checks.

**Step 4: Re-run tests**

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/rule/ddl/table_option_rules.go internal/domain/rule/ddl/table_option_rules_test.go internal/domain/policy/defaults.go configs/deltascope.example.yaml
git commit -m "feat: close create-table object-shape gaps"
```

### Task 6: Verify and close the milestone

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`

**Step 1: Run full validation**

Run:
- `go test ./...`
- `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

Expected: PASS.

**Step 2: Update docs**

Record that create-table coverage is now the create-table superset line relative to `gAudit`, plus any still-open DDL gaps.

**Step 3: Commit**

```bash
git add README.md docs/plans/2026-03-20-deltascope-v1-decisions.md docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md
git commit -m "docs: close create-table superset milestone"
```
