# Rich Alter Semantics Task Prompts

> For task-by-task implementation and review of the `Rich Alter Semantics` milestone.  
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Follow the design in `docs/plans/2026-03-20-rich-alter-semantics-design.md`.
- Follow the implementation sequence in `docs/plans/2026-03-20-rich-alter-semantics-implementation.md`.
- Preserve the DDD-leaning dependency direction:
  - `interfaces -> application -> domain <- infrastructure`
- Do not expose TiDB parser AST types outside `internal/application/audit`.
- Keep all alter rules parser-neutral by consuming domain `spec.Statement` only.
- Continue to support MySQL and TiDB in offline mode only.
- Keep `three-level-doc` as a hard gate:
  - update L2 module README files
  - keep L3 file headers correct
  - run `check_three_level_doc.sh` before claiming completion
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

Expand the domain alter model beyond `Action + Name`.

### Scope

- add richer normalized alter structures
- keep the model minimal and domain-owned

### Files

- Modify `internal/domain/spec/ddl.go`
- Modify `internal/domain/spec/README.md`

### Acceptance Criteria

- `spec.Alter` can represent richer alter semantics
- new alter-related domain structs/types are parser-neutral
- domain README reflects the new shape

### Required Validation

- Run `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

### Reviewer Return

- commit hash
- commit message
- note whether the domain model stayed lean

## Task 2 Prompt

### Goal

Enrich alter extraction in the application layer.

### Scope

- map alter AST into richer normalized domain alter records
- cover column, index, rename, and option-change cases

### Files

- Modify `internal/application/audit/extract.go`
- Modify `internal/application/audit/extract_test.go`
- Modify `internal/application/audit/README.md`

### Acceptance Criteria

- extraction supports representative `ALTER TABLE` shapes
- no TiDB AST leakage escapes the application layer
- tests cover the new alter detail

### Required Validation

- Run `go test ./internal/application/audit -run TestExtractMapsAlterTable -v`

### Reviewer Return

- commit hash
- commit message
- short note on extraction boundary quality

## Task 3 Prompt

### Goal

Prepare the DDL rule layer for richer alter semantics.

### Scope

- add alter rule IDs
- add shared alter helper functions
- document the new rule surface

### Files

- Modify `internal/domain/rule/ddl/common.go`
- Modify `internal/domain/rule/ddl/config.go`
- Modify `internal/domain/rule/ddl/README.md`

### Acceptance Criteria

- alter rule IDs are stable and well-named
- helper functions support richer alter matching without AST access
- DDL module docs describe the new alter rule surface

### Required Validation

- Run `go test ./internal/domain/rule/ddl -run 'TestRegister.*Alter.*' -v`

### Reviewer Return

- commit hash
- commit message
- note whether helper boundaries look reusable

## Task 4 Prompt

### Goal

Implement the first semantic alter rule batch.

### Scope

- type-change rules
- rename-related rules
- policy defaults and config template updates

### Files

- Create `internal/domain/rule/ddl/alter_semantic_rules.go`
- Create `internal/domain/rule/ddl/alter_semantic_rules_test.go`
- Modify `internal/domain/rule/ddl/register.go`
- Modify `internal/domain/policy/defaults.go`
- Modify `internal/domain/policy/README.md`
- Modify `configs/deltascope.example.yaml`

### Acceptance Criteria

- semantic alter rules have stable `rule_id`s
- defaults and example config are aligned with `config init`
- focused tests cover incompatible vs allowed alter cases

### Required Validation

- Run `go test ./internal/domain/rule/ddl -run 'TestAlter.*|TestRegister.*Alter.*' -v`
- Run `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`

### Reviewer Return

- commit hash
- commit message
- list of implemented alter rule IDs

## Task 5 Prompt

### Goal

Extend alter rules to cover alter-added indexes.

### Scope

- reuse existing index-governance logic where possible
- audit alter-added secondary/unique/fulltext indexes

### Files

- Modify `internal/domain/rule/ddl/alter_semantic_rules.go`
- Modify `internal/domain/rule/ddl/alter_semantic_rules_test.go`

### Acceptance Criteria

- alter-added indexes inherit relevant prefix/width checks
- no duplicate create-table rule logic is copied unnecessarily
- focused tests cover the added behavior

### Required Validation

- Run `go test ./internal/domain/rule/ddl -run 'TestAlter.*Index.*' -v`

### Reviewer Return

- commit hash
- commit message
- note whether logic reuse stayed clean

## Task 6 Prompt

### Goal

Close the milestone with full verification and docs.

### Scope

- run full validation
- update top-level project docs and handoff docs

### Files

- Modify `README.md`
- Modify `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify `docs/plans/2026-03-20-overnight-handoff.md`
- Modify `docs/plans/2026-03-20-autonomous-progress.md`

### Acceptance Criteria

- full test suite passes
- config example matches `config init`
- three-level-doc check passes
- docs reflect richer alter semantics and remaining gaps

### Required Validation

- Run `go test ./...`
- Run `diff -u configs/deltascope.example.yaml <(go run ./cmd/deltascope config init)`
- Run `/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh`

### Reviewer Return

- commit hash
- commit message
- note whether the milestone is doc-complete
