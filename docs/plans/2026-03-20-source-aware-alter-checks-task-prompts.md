# Source-Aware Alter Checks Task Prompts

> For task-by-task implementation and review of the `Source-Aware Alter Checks` milestone.  
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Follow the design in `docs/plans/2026-03-20-source-aware-alter-checks-design.md`.
- Follow the implementation sequence in `docs/plans/2026-03-20-source-aware-alter-checks-implementation.md`.
- Preserve the DDD-leaning dependency direction:
  - `interfaces -> application -> domain <- infrastructure`
- Do not expose TiDB parser AST outside `internal/application/audit`.
- Keep all alter rules parser-neutral by consuming domain `spec.Statement` only.
- Continue to support MySQL and TiDB in offline mode only.
- Keep `three-level-doc` as a hard gate.
- Reviewer-facing completion for every task must include:
  - files changed
  - tests run
  - status
  - git commit hash

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

Enrich alter domain facts for source-aware change judgment.

### Scope

- add only the minimal relation-aware fields needed by downstream rules
- keep the model parser-neutral and domain-owned

### Files

- Modify `internal/domain/spec/ddl.go`
- Modify `internal/domain/spec/README.md`

### Acceptance Criteria

- alter payloads can describe source-aware column-change facts
- the domain model stays lean and parser-neutral
- module docs reflect the new shape

### Required Validation

- Run `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

## Task 2 Prompt

### Goal

Extract source-aware alter facts in the application layer.

### Scope

- extend alter extraction without leaking TiDB AST
- cover relation-aware column-change and alter-index detail

### Files

- Modify `internal/application/audit/extract.go`
- Modify `internal/application/audit/extract_test.go`
- Modify `internal/application/audit/README.md`

### Acceptance Criteria

- representative `ALTER TABLE` shapes produce richer parser-neutral facts
- no TiDB AST escapes the application layer
- tests cover the new extraction detail

### Required Validation

- Run `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

## Task 3 Prompt

### Goal

Prepare the DDL rule layer for source-aware alter policies.

### Scope

- add stable source-aware alter rule IDs
- add shared helpers for change comparison and alter-index projection
- document the new helper/rule surface

### Files

- Modify `internal/domain/rule/ddl/common.go`
- Modify `internal/domain/rule/ddl/config.go`
- Modify `internal/domain/rule/ddl/README.md`

### Acceptance Criteria

- rule IDs are stable and honest
- helpers support source-aware alter judgment without AST access
- DDL module docs describe the new surface

### Required Validation

- Run `go test ./internal/domain/rule/ddl -run 'TestRegister.*Alter.*' -v`

## Task 4 Prompt

### Goal

Implement source-aware explicit column alter rules.

### Scope

- target-side type-family allowlists
- explicit nullability/default/auto_increment change checks
- defaults and config template updates

### Files

- Modify `internal/domain/rule/ddl/alter_semantic_rules.go`
- Modify `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- Modify `internal/domain/rule/ddl/register.go`
- Modify `internal/domain/policy/defaults.go`
- Modify `internal/domain/policy/README.md`
- Modify `configs/deltascope.example.yaml`

### Acceptance Criteria

- rule IDs stay honest about what is actually judged
- defaults and example config align with `config init`
- focused tests cover blocked vs allowed explicit-change cases

### Required Validation

- Run `go test ./internal/domain/rule/ddl -run 'TestAlter.*(Column|Transition|Register).*' -v`
- Run `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`

## Task 5 Prompt

### Goal

Extend alter-index lifecycle governance.

### Scope

- reuse existing index rule logic where possible
- cover alter-added width/duplicate checks and supported rename/drop cases

### Files

- Modify `internal/domain/rule/ddl/alter_semantic_rules.go`
- Modify `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- Modify `internal/domain/rule/ddl/register.go`

### Acceptance Criteria

- alter-index lifecycle checks reuse existing logic cleanly
- no duplicate create-table rule bodies are copied unnecessarily
- focused tests cover the added behavior

### Required Validation

- Run `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`

## Task 6 Prompt

### Goal

Close the milestone with full verification and docs.

### Scope

- run full validation
- update top-level docs and handoff docs

### Files

- Modify `README.md`
- Modify `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify `docs/plans/2026-03-20-overnight-handoff.md`
- Modify `docs/plans/2026-03-20-autonomous-progress.md`

### Acceptance Criteria

- full test suite passes
- config example matches `config init`
- three-level-doc check passes
- docs reflect new source-aware alter behavior and remaining create-table gaps

### Required Validation

- Run `go test ./...`
- Run `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- Run `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`
