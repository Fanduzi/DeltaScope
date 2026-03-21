# Source-Aware Alter Checks Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** deepen offline `ALTER TABLE` auditing with source-aware column-change semantics and stronger alter-index lifecycle checks.

**Architecture:** keep TiDB AST inside `internal/application/audit`, enrich parser-neutral alter facts in `internal/domain/spec`, and implement all judgment in `internal/domain/rule/ddl` through domain-only helpers and rules.

**Tech Stack:** Go, TiDB parser AST, Cobra/Viper config flow, Go testing

---

### Task 1: Enrich alter domain facts for source-aware change judgment

**Files:**
- Modify: `internal/domain/spec/ddl.go`
- Modify: `internal/domain/spec/README.md`

**Step 1: Add failing expectations in downstream extractor/rule tests**

Expect alter payloads to expose enough detail for:
- source vs target type family
- nullability/default/unsigned changes
- rename-plus-change compound cases

**Step 2: Run the targeted tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: FAIL.

**Step 3: Add the minimal domain shape**

Add only the relation-aware alter fields needed by the first source-aware rule batch.

**Step 4: Re-run the targeted tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: still FAIL until extraction is updated.

**Step 5: Commit**

```bash
git add internal/domain/spec/ddl.go internal/domain/spec/README.md
git commit -m "refactor: enrich alter change facts"
```

### Task 2: Extend alter extraction for source-aware facts

**Files:**
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Modify: `internal/application/audit/README.md`

**Step 1: Write failing extractor cases**

Cover:
- modify/change with explicit target definitions
- rename plus type/default/nullability changes
- alter-added index width cases

**Step 2: Run the targeted extractor tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: FAIL.

**Step 3: Implement minimal extraction**

Populate parser-neutral change facts only from statement-local data.

**Step 4: Re-run the extractor tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/application/audit/README.md
git commit -m "feat: extract source-aware alter facts"
```

### Task 3: Prepare rule helpers for source-aware alter policies

**Files:**
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/config.go`
- Modify: `internal/domain/rule/ddl/README.md`

**Step 1: Add the new rule IDs**

Introduce stable IDs for:
- source-to-target compatibility
- nullability/default transition checks
- alter-added index width and duplicate checks

**Step 2: Add shared helper functions**

Helpers should cover:
- comparing source/target column facts
- classifying transition kinds
- projecting alter-added indexes into reusable create-table index rule inputs

**Step 3: Add/update docs**

Document the new rule surface and helper intent.

**Step 4: Commit**

```bash
git add internal/domain/rule/ddl/common.go internal/domain/rule/ddl/config.go internal/domain/rule/ddl/README.md
git commit -m "refactor: prepare source-aware alter rules"
```

### Task 4: Implement source-aware explicit column alter rules

**Files:**
- Modify: `internal/domain/rule/ddl/alter_semantic_rules.go`
- Modify: `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/policy/defaults.go`
- Modify: `internal/domain/policy/README.md`
- Modify: `configs/deltascope.example.yaml`

**Step 1: Write failing rule tests**

Cover:
- target-side type-family allowlists
- explicit nullability/default/auto-increment changes
- blocked vs allowed explicit-change cases

**Step 2: Run the targeted DDL tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*(Column|Transition|Register).*' -v`
Expected: FAIL.

**Step 3: Implement the minimal rules**

Keep the offline semantics conservative and explicit. Do not invent an unsigned-transition rule unless the model can honestly prove that transition from statement-local facts.

**Step 4: Align defaults and config template**

Update default policy plus `configs/deltascope.example.yaml`.

**Step 5: Re-run the targeted DDL tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*(Column|Transition|Register).*' -v`
Expected: PASS.

**Step 6: Commit**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go internal/domain/rule/ddl/register.go internal/domain/policy/defaults.go internal/domain/policy/README.md configs/deltascope.example.yaml
git commit -m "feat: add explicit alter column rules"
```

### Task 5: Extend alter-index lifecycle governance

**Files:**
- Modify: `internal/domain/rule/ddl/alter_semantic_rules.go`
- Modify: `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- Modify: `internal/domain/rule/ddl/register.go`

**Step 1: Write failing tests**

Cover:
- alter-added index width checks
- alter-added duplicate-index checks
- stronger rename/drop index cases if supported by existing facts

**Step 2: Run the targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
Expected: FAIL.

**Step 3: Implement minimal logic**

Reuse existing create-table index rule bodies where possible.

**Step 4: Re-run the targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go internal/domain/rule/ddl/register.go
git commit -m "feat: deepen alter index lifecycle checks"
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

Record:
- what source-aware alter checks now exist
- which gaps moved to the create-table superset milestone
- key commits and tradeoffs

**Step 3: Commit**

```bash
git add README.md docs/plans/2026-03-20-deltascope-v1-decisions.md docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md
git commit -m "docs: close source-aware alter checks milestone"
```
