# DeltaScope v1 Task Prompts

> For task-by-task implementation and review.  
> Every task prompt is designed for an execution agent working inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Follow the design in `docs/plans/2026-03-20-deltascope-v1-design.md`.
- Follow the implementation sequence in `docs/plans/2026-03-20-deltascope-v1-implementation.md`.
- Do not mirror `gAudit` package layout or checker style.
- Keep the architecture DDD-leaning:
  - `interfaces -> application -> domain <- infrastructure`
- The domain layer must not depend on Cobra, Viper, or TiDB parser AST types.
- `DeltaScope` v1 is offline-only and must not require a live database connection.
- Supported dialects are MySQL and TiDB.
- CLI requirements:
  - Cobra
  - Viper
  - YAML config
  - default Markdown output
  - required `--format json`
- Findings levels:
  - `blocker`
  - `warning`
  - `notice`
- Reviewer-facing result for every task must include:
  - summary of files changed
  - tests run
  - current status
  - git commit hash after committing

## Reviewer Response Template

Use this exact structure when reporting task completion:

```md
Task: <task name>

What changed:
- ...

Validation:
- Ran: `<command>`
- Result: PASS/FAIL

Commit:
- Hash: `<commit hash>`
- Message: `<commit message>`

Open questions:
- None
```

## Task 1 Prompt

### Goal

Initialize the repository skeleton for `DeltaScope`.

### Scope

- Create the Go module
- Create the package skeleton
- Add a minimal CLI bootstrap entrypoint

### Files

- Create `go.mod`
- Create `cmd/deltascope/main.go`
- Create `internal/interfaces/cli/`
- Create `internal/application/`
- Create `internal/domain/`
- Create `internal/infrastructure/`
- Create `pkg/deltascope/`

### Acceptance Criteria

- `go.mod` exists
- the package layout matches the implementation plan
- `cmd/deltascope/main.go` exists and delegates to CLI code
- `go test ./...` completes successfully with placeholders

### Required Validation

- Run `go test ./...`

### Reviewer Return

- commit hash
- commit message
- note whether the skeleton is compile-clean

## Task 2 Prompt

### Goal

Define the first core domain types for statements, policy, findings, verdict, and result aggregation.

### Scope

- Add domain model skeletons
- Add verdict aggregation behavior
- Add tests for `pass`, `review`, and `reject`

### Files

- Create `internal/domain/spec/statement.go`
- Create `internal/domain/spec/ddl.go`
- Create `internal/domain/spec/dml.go`
- Create `internal/domain/rule/rule.go`
- Create `internal/domain/report/result.go`
- Create `internal/domain/policy/policy.go`
- Create `internal/domain/report/result_test.go`

### Acceptance Criteria

- domain types exist with clear ownership
- finding levels are `blocker`, `warning`, `notice`
- verdict logic is covered by tests
- tests pass for report aggregation

### Required Validation

- Run `go test ./internal/domain/report -run TestVerdict -v`

### Reviewer Return

- commit hash
- commit message
- short note on domain boundary quality

## Task 3 Prompt

### Goal

Implement default policy and YAML loading with Viper.

### Scope

- built-in default policy
- YAML file override support
- example config file

### Files

- Create `internal/infrastructure/config/viper/loader.go`
- Create `internal/application/policy/load.go`
- Create `internal/domain/policy/defaults.go`
- Create `configs/deltascope.example.yaml`
- Create `internal/infrastructure/config/viper/loader_test.go`

### Acceptance Criteria

- defaults load when no config file is supplied
- YAML overrides are unmarshaled correctly
- rule IDs map cleanly to policy entries
- example config matches the intended rule naming style

### Required Validation

- Run `go test ./internal/infrastructure/config/viper -run TestLoader -v`

### Reviewer Return

- commit hash
- commit message
- note whether config format is stable enough for users

## Task 4 Prompt

### Goal

Add the TiDB parser adapter and statement classification support.

### Scope

- parse multi-statement SQL
- surface parser warnings and failures cleanly
- avoid leaking AST types into domain

### Files

- Create `internal/infrastructure/parser/tidb/parser.go`
- Create `internal/application/audit/parse.go`
- Create `internal/domain/spec/kind.go`
- Create `internal/infrastructure/parser/tidb/parser_test.go`

### Acceptance Criteria

- parser adapter can parse valid multi-statement SQL
- parse failures are test-covered
- AST is contained within infrastructure/application boundaries

### Required Validation

- Run `go test ./internal/infrastructure/parser/tidb -run TestParser -v`

### Reviewer Return

- commit hash
- commit message
- note whether AST leakage was avoided

## Task 5 Prompt

### Goal

Convert parsed AST into the unified `StatementSpec` model.

### Scope

- implement first-pass extraction for representative DDL and DML statements
- populate core `StatementSpec` fields

### Files

- Create `internal/application/audit/extract.go`
- Create `internal/infrastructure/parser/tidb/extractor.go`
- Create `internal/infrastructure/parser/tidb/extractor_test.go`

### Acceptance Criteria

- `CREATE TABLE`, `ALTER TABLE`, `INSERT`, `UPDATE`, and `DELETE` are mapped
- `StatementSpec` includes kind, raw SQL, normalized SQL, and first-pass substructures
- tests cover extraction behavior

### Required Validation

- Run `go test ./internal/infrastructure/parser/tidb -run TestExtractor -v`

### Reviewer Return

- commit hash
- commit message
- note which statement shapes are supported

## Task 6 Prompt

### Goal

Build the core rule engine and registry.

### Scope

- rule registration
- applicability filtering
- finding collection

### Files

- Create `internal/domain/rule/registry.go`
- Create `internal/application/audit/evaluate.go`
- Create `internal/domain/rule/registry_test.go`

### Acceptance Criteria

- rules can be registered and evaluated deterministically
- statement-level applicability works
- findings are collected into the report flow

### Required Validation

- Run `go test ./internal/domain/rule -run TestRegistry -v`

### Reviewer Return

- commit hash
- commit message
- note whether the engine is extensible enough for Tier-1 rules

## Task 7 Prompt

### Goal

Implement Tier-1 DDL rules.

### Scope

- table naming and comments
- primary key shape
- audit columns
- column constraints
- index constraints
- alter restrictions

### Files

- Create rule files under `internal/domain/rule/ddl/`
- Create tests under `internal/domain/rule/ddl/`

### Acceptance Criteria

- DDL rules have stable `rule_id`s
- rule messages and levels are deterministic
- each concern is covered by focused tests
- rules align with the v1 design and improve on `gAudit` structure

### Required Validation

- Run `go test ./internal/domain/rule/ddl/... -v`

### Reviewer Return

- commit hash
- commit message
- list of implemented rule IDs

## Task 8 Prompt

### Goal

Implement Tier-1 DML rules.

### Scope

- require `WHERE`
- forbid `LIMIT`
- forbid `ORDER BY`
- forbid subqueries
- require `JOIN ... ON`
- limit insert row count
- forbid `REPLACE`
- forbid `INSERT ... SELECT`
- forbid `ON DUPLICATE KEY`

### Files

- Create rule files under `internal/domain/rule/dml/`
- Create tests under `internal/domain/rule/dml/`

### Acceptance Criteria

- DML rules are independently testable
- rule IDs and severities align with policy design
- representative SQL fixtures are covered

### Required Validation

- Run `go test ./internal/domain/rule/dml/... -v`

### Reviewer Return

- commit hash
- commit message
- list of implemented rule IDs

## Task 9 Prompt

### Goal

Assemble the application audit use case and stable public library API.

### Scope

- application service to orchestrate policy, parsing, extraction, rules, and reporting
- stable `pkg/deltascope` entrypoint

### Files

- Create `internal/application/audit/service.go`
- Create `pkg/deltascope/audit.go`
- Create `pkg/deltascope/audit_test.go`

### Acceptance Criteria

- public API can audit inline SQL with default policy
- config override path works
- multi-statement SQL returns grouped statement results
- result includes verdict and statement findings

### Required Validation

- Run `go test ./pkg/deltascope -run TestAudit -v`

### Reviewer Return

- commit hash
- commit message
- short note on public API stability

## Task 10 Prompt

### Goal

Add Markdown and JSON renderers for audit results.

### Scope

- default Markdown renderer
- stable JSON renderer for machine consumption

### Files

- Create `internal/infrastructure/output/markdown/render.go`
- Create `internal/infrastructure/output/json/render.go`
- Create renderer tests

### Acceptance Criteria

- Markdown is easy for humans and AI agents to scan
- JSON keys are stable and machine-oriented
- renderers pass targeted tests

### Required Validation

- Run `go test ./internal/infrastructure/output/... -v`

### Reviewer Return

- commit hash
- commit message
- note whether JSON is stable enough for skill integration

## Task 11 Prompt

### Goal

Build the Cobra CLI for `DeltaScope`.

### Scope

- `audit`
- `config init`
- `version`
- flag wiring and exit code behavior

### Files

- Create `internal/interfaces/cli/root.go`
- Create `internal/interfaces/cli/audit.go`
- Create `internal/interfaces/cli/config_init.go`
- Create `internal/interfaces/cli/version.go`
- Modify `cmd/deltascope/main.go`
- Create CLI tests

### Acceptance Criteria

- supports `--sql`, `--file`, and stdin
- supports `--format json`
- supports `--config`, `--dialect`, `--format`, `--fail-on`, `--quiet`
- `config init` generates a usable YAML template
- exit codes follow the agreed contract

### Required Validation

- Run `go test ./internal/interfaces/cli/... -v`

### Reviewer Return

- commit hash
- commit message
- sample CLI invocation used for manual sanity-check

## Task 12 Prompt

### Goal

Finalize documentation and verify the repository end to end.

### Scope

- write README
- verify examples
- run the full test suite

### Files

- Create `README.md`
- Modify `configs/deltascope.example.yaml`
- Update plan/design docs if implementation diverged in justified ways

### Acceptance Criteria

- README explains what `DeltaScope` is and how to use it
- example config is consistent with implementation
- full repository tests pass
- manual CLI smoke test works in both Markdown and JSON modes

### Required Validation

- Run `go test ./...`
- Run `go run ./cmd/deltascope audit --sql "delete from t"`
- Run `go run ./cmd/deltascope audit --sql "delete from t" --format json`

### Reviewer Return

- commit hash
- commit message
- note any remaining gaps before HTTP API work
