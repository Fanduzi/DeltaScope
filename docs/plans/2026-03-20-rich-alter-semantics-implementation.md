# Rich Alter Semantics Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** upgrade `ALTER TABLE` from coarse action-only auditing to a richer normalized domain model plus a first batch of semantic alter rules.

**Architecture:** preserve `internal/application/audit` as the only AST-aware layer, enrich `internal/domain/spec.Alter` with typed detail records, and keep all alter auditing inside `internal/domain/rule/ddl` against parser-neutral domain structures.

**Tech Stack:** Go, TiDB parser AST, Cobra/Viper config flow, Go testing

---

### Task 1: Expand the domain alter model

**Files:**
- Modify: `internal/domain/spec/ddl.go`
- Modify: `internal/domain/spec/README.md`

**Step 1: Add failing expectations in downstream tests**

Update upcoming extractor/rule tests so they require richer alter detail such as:
- old/new column name
- normalized target type
- index kind and column list
- changed table options

**Step 2: Run the targeted tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: FAIL once the tests expect the new detail.

**Step 3: Add minimal domain structs**

Add:
- `AlterAction` type
- `AlterColumn`
- `AlterIndex`
- richer `Alter`

Keep only fields needed by the first rule batch.

**Step 4: Re-run the targeted tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: still FAIL until extraction is implemented.

**Step 5: Commit**

```bash
git add internal/domain/spec/ddl.go internal/domain/spec/README.md
git commit -m "refactor: expand alter domain model"
```

### Task 2: Enrich alter extraction

**Files:**
- Modify: `internal/application/audit/extract.go`
- Modify: `internal/application/audit/extract_test.go`
- Modify: `internal/application/audit/README.md`

**Step 1: Write the failing extractor cases**

Cover:
- `ALTER TABLE ... MODIFY COLUMN`
- `ALTER TABLE ... CHANGE COLUMN`
- `ALTER TABLE ... RENAME COLUMN`
- `ALTER TABLE ... ADD INDEX`
- `ALTER TABLE ... DROP INDEX`
- `ALTER TABLE ... RENAME INDEX`
- `ALTER TABLE ... ENGINE=...`

**Step 2: Run the targeted extractor tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: FAIL.

**Step 3: Implement minimal extraction**

Populate richer `Alter` detail without exposing TiDB AST outside the application layer.

**Step 4: Re-run the extractor tests**

Run: `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/application/audit/extract.go internal/application/audit/extract_test.go internal/application/audit/README.md
git commit -m "feat: enrich alter extraction"
```

### Task 3: Add alter semantic rule constructors and shared helpers

**Files:**
- Modify: `internal/domain/rule/ddl/common.go`
- Modify: `internal/domain/rule/ddl/config.go`
- Modify: `internal/domain/rule/ddl/README.md`

**Step 1: Define the new rule IDs**

Add IDs for the first richer alter batch, for example:
- `ddl.alter.modify_column.compatible.require`
- `ddl.alter.rename_index.forbid`
- `ddl.alter.add_index.secondary.prefix.require`

**Step 2: Add shared helper functions**

Helpers should cover:
- alter applicability checks
- locating rename/type-change details
- compatibility classification

**Step 3: Add/update module docs**

Document the new alter rule surface in the module README.

**Step 4: Commit**

```bash
git add internal/domain/rule/ddl/common.go internal/domain/rule/ddl/config.go internal/domain/rule/ddl/README.md
git commit -m "refactor: prepare alter semantic rules"
```

### Task 4: Implement type-change and rename alter rules

**Files:**
- Create: `internal/domain/rule/ddl/alter_semantic_rules.go`
- Create: `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- Modify: `internal/domain/rule/ddl/register.go`
- Modify: `internal/domain/policy/defaults.go`
- Modify: `internal/domain/policy/README.md`
- Modify: `configs/deltascope.example.yaml`

**Step 1: Write failing domain-rule tests**

Cover:
- incompatible modify/change type
- compatible type change allowed path
- rename column forbid
- rename index forbid

**Step 2: Run the targeted DDL tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*|TestRegister.*Alter.*' -v`
Expected: FAIL.

**Step 3: Implement the minimal rules**

Keep rules focused and parser-neutral.

**Step 4: Wire defaults and config template**

Add new defaults and keep `configs/deltascope.example.yaml` aligned with `config init`.

**Step 5: Re-run the targeted DDL tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*|TestRegister.*Alter.*' -v`
Expected: PASS.

**Step 6: Commit**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go internal/domain/rule/ddl/register.go internal/domain/policy/defaults.go internal/domain/policy/README.md configs/deltascope.example.yaml
git commit -m "feat: add alter semantic rules"
```

### Task 5: Reuse create-table index rules for alter-added indexes where possible

**Files:**
- Modify: `internal/domain/rule/ddl/alter_semantic_rules.go`
- Modify: `internal/domain/rule/ddl/alter_semantic_rules_test.go`

**Step 1: Add failing tests for alter-added indexes**

Cover:
- secondary index prefix
- index width / key-part count

**Step 2: Run the targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
Expected: FAIL.

**Step 3: Implement minimal logic**

Reuse existing index-governance helpers when possible instead of duplicating rule logic.

**Step 4: Re-run the targeted tests**

Run: `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/domain/rule/ddl/alter_semantic_rules.go internal/domain/rule/ddl/alter_semantic_rules_test.go
git commit -m "feat: extend alter rules for added indexes"
```

### Task 6: Final verification and docs

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`

**Step 1: Run the full verification suite**

Run:
- `go test ./...`
- `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

Expected: PASS.

**Step 2: Update top-level docs**

Record:
- what richer alter semantics now cover
- what remains deferred
- which new commits landed

**Step 3: Commit**

```bash
git add README.md docs/plans/2026-03-20-deltascope-v1-decisions.md docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md
git commit -m "docs: record rich alter semantics milestone"
```
