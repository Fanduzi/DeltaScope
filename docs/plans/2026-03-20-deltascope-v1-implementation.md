# DeltaScope v1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the first working `DeltaScope` release as an offline MySQL/TiDB DDL-DML review library plus Cobra CLI with YAML policy support and stable Markdown/JSON output.

**Architecture:** Use a DDD-leaning structure where the domain owns `StatementSpec`, rules, policy, report aggregation, and verdict logic. Infrastructure adapts TiDB parser, Viper config loading, and output rendering; the CLI remains a thin interface over the application use case.

**Tech Stack:** Go, Cobra, Viper, TiDB parser, fsnotify, Go testing

---

### Task 1: Initialize the Repository Skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/deltascope/main.go`
- Create: `internal/interfaces/cli/`
- Create: `internal/application/`
- Create: `internal/domain/`
- Create: `internal/infrastructure/`
- Create: `pkg/deltascope/`

**Step 1: Initialize Go module**

Run: `go mod init github.com/fan/deltascope`
Expected: `go.mod` is created with the new module path.

**Step 2: Create empty package skeleton**

Create the core directories for interfaces, application, domain, infrastructure, and public package entry.

**Step 3: Add a minimal `main.go`**

Implement a small CLI bootstrap that delegates to the CLI package.

**Step 4: Verify compilation baseline**

Run: `go test ./...`
Expected: packages compile with placeholder implementations.

**Step 5: Commit**

```bash
git add .
git commit -m "chore: initialize deltascope project skeleton"
```

### Task 2: Define Core Domain Types

**Files:**
- Create: `internal/domain/spec/statement.go`
- Create: `internal/domain/spec/ddl.go`
- Create: `internal/domain/spec/dml.go`
- Create: `internal/domain/rule/rule.go`
- Create: `internal/domain/report/result.go`
- Create: `internal/domain/policy/policy.go`
- Test: `internal/domain/report/result_test.go`

**Step 1: Write failing verdict aggregation tests**

Cover `pass`, `review`, and `reject` behavior from notice/warning/blocker findings.

**Step 2: Run targeted tests**

Run: `go test ./internal/domain/report -run TestVerdict -v`
Expected: FAIL because types and aggregation do not exist yet.

**Step 3: Implement minimal domain objects**

Define `StatementSpec`, finding levels, verdict calculation, summary counts, and policy stubs.

**Step 4: Re-run tests**

Run: `go test ./internal/domain/report -run TestVerdict -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: define core audit domain types"
```

### Task 3: Implement Policy Defaults and YAML Loading

**Files:**
- Create: `internal/infrastructure/config/viper/loader.go`
- Create: `internal/application/policy/load.go`
- Create: `internal/domain/policy/defaults.go`
- Create: `configs/deltascope.example.yaml`
- Test: `internal/infrastructure/config/viper/loader_test.go`

**Step 1: Write failing config loader tests**

Cover built-in defaults, YAML override loading, and rule-level value/level parsing.

**Step 2: Run targeted tests**

Run: `go test ./internal/infrastructure/config/viper -run TestLoader -v`
Expected: FAIL because loader is missing.

**Step 3: Implement Viper-backed loading**

Support default policy when no file is passed and file-based overrides when a YAML path is supplied.

**Step 4: Add example config**

Create a human-editable example matching the chosen rule ID scheme.

**Step 5: Re-run tests**

Run: `go test ./internal/infrastructure/config/viper -run TestLoader -v`
Expected: PASS.

**Step 6: Commit**

```bash
git add .
git commit -m "feat: add YAML policy loading with defaults"
```

### Task 4: Add Parser Adapter and Statement Classification

**Files:**
- Create: `internal/infrastructure/parser/tidb/parser.go`
- Create: `internal/application/audit/parse.go`
- Create: `internal/domain/spec/kind.go`
- Test: `internal/infrastructure/parser/tidb/parser_test.go`

**Step 1: Write failing parser tests**

Cover successful parsing of multi-statement SQL and parse failure behavior.

**Step 2: Run targeted tests**

Run: `go test ./internal/infrastructure/parser/tidb -run TestParser -v`
Expected: FAIL because parser adapter is missing.

**Step 3: Implement TiDB parser adapter**

Return parsed statements plus parser warnings in an infrastructure-neutral form.

**Step 4: Re-run tests**

Run: `go test ./internal/infrastructure/parser/tidb -run TestParser -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add TiDB parser adapter"
```

### Task 5: Extract AST into `StatementSpec`

**Files:**
- Create: `internal/application/audit/extract.go`
- Create: `internal/infrastructure/parser/tidb/extractor.go`
- Test: `internal/infrastructure/parser/tidb/extractor_test.go`

**Step 1: Write failing extractor tests**

Cover representative statements:
- `CREATE TABLE`
- `ALTER TABLE`
- `INSERT`
- `UPDATE`
- `DELETE`

**Step 2: Run targeted tests**

Run: `go test ./internal/infrastructure/parser/tidb -run TestExtractor -v`
Expected: FAIL because extraction is not implemented.

**Step 3: Implement minimal `StatementSpec` extraction**

Populate statement kind, raw SQL, normalized SQL, and the first-pass DDL/DML sub-structures.

**Step 4: Re-run tests**

Run: `go test ./internal/infrastructure/parser/tidb -run TestExtractor -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: map AST statements into StatementSpec"
```

### Task 6: Build the Rule Engine Core

**Files:**
- Create: `internal/domain/rule/registry.go`
- Create: `internal/application/audit/evaluate.go`
- Test: `internal/domain/rule/registry_test.go`

**Step 1: Write failing rule engine tests**

Cover rule registration, applicability filtering, and finding collection.

**Step 2: Run targeted tests**

Run: `go test ./internal/domain/rule -run TestRegistry -v`
Expected: FAIL because the registry and evaluator do not exist.

**Step 3: Implement the minimal engine**

Add rule registration, statement applicability checks, and collection of statement/global findings.

**Step 4: Re-run tests**

Run: `go test ./internal/domain/rule -run TestRegistry -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add core rule engine"
```

### Task 7: Implement Tier-1 DDL Rules

**Files:**
- Create: `internal/domain/rule/ddl/...`
- Test: `internal/domain/rule/ddl/..._test.go`

**Step 1: Group DDL rules into small batches**

Implement in batches with one test file per concern:
- table naming and comments
- primary key shape
- audit columns
- column constraints
- index constraints
- alter restrictions

**Step 2: Write failing tests per batch**

Use small SQL fixtures and assert stable `rule_id`, `level`, and message behavior.

**Step 3: Implement the minimal rules for that batch**

Do not mix unrelated checks in one rule file.

**Step 4: Re-run focused tests after each batch**

Run: `go test ./internal/domain/rule/ddl/... -v`
Expected: PASS.

**Step 5: Commit after each completed batch**

Example:

```bash
git add .
git commit -m "feat: add DDL primary key and audit column rules"
```

### Task 8: Implement Tier-1 DML Rules

**Files:**
- Create: `internal/domain/rule/dml/...`
- Test: `internal/domain/rule/dml/..._test.go`

**Step 1: Write failing tests for DML shape rules**

Cover:
- missing `WHERE`
- forbidden `LIMIT`
- forbidden `ORDER BY`
- forbidden subqueries
- join without `ON`
- insert row count
- replace / insert-select / on-duplicate restrictions

**Step 2: Run focused tests**

Run: `go test ./internal/domain/rule/dml/... -v`
Expected: FAIL initially.

**Step 3: Implement DML rules in small batches**

Keep rule ID naming and severity defaults aligned with policy design.

**Step 4: Re-run focused tests**

Run: `go test ./internal/domain/rule/dml/... -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add Tier-1 DML rules"
```

### Task 9: Assemble the Application Use Case and Public API

**Files:**
- Create: `internal/application/audit/service.go`
- Create: `pkg/deltascope/audit.go`
- Test: `pkg/deltascope/audit_test.go`

**Step 1: Write failing end-to-end library tests**

Cover:
- default policy with inline SQL
- config override path
- multi-statement SQL
- verdict and statement grouping

**Step 2: Run targeted tests**

Run: `go test ./pkg/deltascope -run TestAudit -v`
Expected: FAIL because the public API does not exist.

**Step 3: Implement the use case**

Wire policy loading, parsing, extraction, evaluation, and report aggregation into one stable public function.

**Step 4: Re-run tests**

Run: `go test ./pkg/deltascope -run TestAudit -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: expose public audit API"
```

### Task 10: Add Markdown and JSON Renderers

**Files:**
- Create: `internal/infrastructure/output/markdown/render.go`
- Create: `internal/infrastructure/output/json/render.go`
- Test: `internal/infrastructure/output/..._test.go`

**Step 1: Write failing renderer tests**

Cover stable verdict, summary, and statement finding rendering for both formats.

**Step 2: Run targeted tests**

Run: `go test ./internal/infrastructure/output/... -v`
Expected: FAIL because renderers do not exist.

**Step 3: Implement Markdown and JSON formatting**

Keep JSON keys stable and Markdown sections easy to scan by humans and AI agents.

**Step 4: Re-run tests**

Run: `go test ./internal/infrastructure/output/... -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "feat: add markdown and json renderers"
```

### Task 11: Build the Cobra CLI

**Files:**
- Create: `internal/interfaces/cli/root.go`
- Create: `internal/interfaces/cli/audit.go`
- Create: `internal/interfaces/cli/config_init.go`
- Create: `internal/interfaces/cli/version.go`
- Modify: `cmd/deltascope/main.go`
- Test: `internal/interfaces/cli/..._test.go`

**Step 1: Write failing CLI tests**

Cover:
- `audit --sql`
- `audit --file`
- stdin input
- `--format json`
- `config init`
- exit code threshold behavior

**Step 2: Run targeted tests**

Run: `go test ./internal/interfaces/cli/... -v`
Expected: FAIL before the commands are implemented.

**Step 3: Implement Cobra commands**

Add `audit`, `config init`, and `version` as the v1 command set.

**Step 4: Bind config-related flags cleanly**

Wire `--config`, `--dialect`, `--format`, `--fail-on`, and `--quiet`.

**Step 5: Re-run tests**

Run: `go test ./internal/interfaces/cli/... -v`
Expected: PASS.

**Step 6: Commit**

```bash
git add .
git commit -m "feat: add deltascope cobra cli"
```

### Task 12: Final Verification and Documentation

**Files:**
- Create: `README.md`
- Modify: `configs/deltascope.example.yaml`
- Modify: `docs/plans/2026-03-20-deltascope-v1-design.md`

**Step 1: Write README**

Document:
- what `DeltaScope` is
- supported dialects
- offline audit model
- CLI examples
- config examples
- output examples

**Step 2: Run full verification**

Run: `go test ./...`
Expected: PASS across the repository.

**Step 3: Smoke-test CLI manually**

Run:
- `go run ./cmd/deltascope audit --sql "delete from t"`
- `go run ./cmd/deltascope audit --sql "delete from t" --format json`

Expected:
- Markdown output for the first command
- valid JSON output for the second

**Step 4: Commit**

```bash
git add .
git commit -m "docs: add DeltaScope README and examples"
```
