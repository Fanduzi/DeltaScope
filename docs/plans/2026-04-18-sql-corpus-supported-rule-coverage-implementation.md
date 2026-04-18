# SQL Corpus Supported-Rule Coverage Implementation Plan

> **For agentic workers:** keep scope narrow. This plan is about corpus fixture capability, coverage gating, and CI/docs integration. It is not a rule-authoring milestone.

**Goal:** make SQL corpus coverage a repository-enforced contract for every currently supported `rule_id × dialect` surface.

**Architecture:** extend the corpus harness to accept inline config and metadata-driven audit inputs, add a rule-surface coverage gate based on `policy.Default()`, populate corpus cases across MySQL/TiDB/PostgreSQL, then route the gate through local Make targets and CI.

**Tech Stack:** Go tests, YAML corpus fixtures, Markdown docs, GitHub Actions, Make.

---

## Scope

### In Scope

- corpus harness widening for inline `config`
- corpus harness widening for YAML-driven metadata provider input
- support-surface coverage gate
- corpus fixture additions for supported rule surfaces
- CI / Make / docs integration for the new gate

### Out of Scope

- new production rule semantics
- extractor/spec widening just to satisfy corpus coverage
- line-coverage instrumentation
- release-note or marketing-surface changes

---

## File Structure

**Harness and gate files:**
- Modify: `internal/application/audit/corpus_testdata_test.go`
- Modify: `internal/application/audit/corpus_helpers_test.go`
- Modify: `internal/application/audit/corpus_test.go`
- Modify: `internal/application/audit/corpus_postgresql_tag_test.go`
- Create: `internal/application/audit/corpus_coverage_test.go`

**Corpus fixtures:**
- Modify/Create: `testdata/sql-corpus/mysql/**`
- Modify/Create: `testdata/sql-corpus/tidb/**`
- Modify/Create: `testdata/sql-corpus/postgresql/**`

**CI / docs:**
- Modify: `Makefile`
- Modify: `.github/workflows/release-smoke.yml`
- Modify: `docs/dev/testing.md`

**Do not modify unless a separate milestone approves it:**
- `internal/domain/rule/**/*.go`
- `internal/infrastructure/parser/**/*.go`
- `internal/domain/spec/**/*.go`
- release notes / changelog / README product surfaces

---

## Task 1: Widen the corpus harness input model

**Files:**
- `internal/application/audit/corpus_testdata_test.go`
- `internal/application/audit/corpus_helpers_test.go`
- `internal/application/audit/corpus_test.go`
- `internal/application/audit/corpus_postgresql_tag_test.go`

- [ ] Add top-level `config:` support to corpus YAML
- [ ] Materialize inline config into a temporary config file for `AuditSQL`
- [ ] Add top-level `metadata:` support with:
  - `schema`
  - `instance`
  - `tables`
  - `index_owners`
- [ ] Implement a fixture-backed metadata provider consumed through normal audit request wiring
- [ ] Keep existing expectation assertions intact

**Acceptance:**
- metadata-aware rules can be triggered from corpus fixtures
- config-sensitive rules can be triggered without external config files
- no production code changes are required

---

## Task 2: Add the supported-rule coverage gate

**Files:**
- `internal/application/audit/corpus_coverage_test.go`

- [ ] Enumerate rule IDs from `policy.Default()`
- [ ] Scan `testdata/sql-corpus/**/*.expected.yaml`
- [ ] Record coverage from `expect.findings.include`
- [ ] Define dialect target mapping:
  - PostgreSQL-only
  - MySQL-family-only
  - shared all-dialect rules
  - explicitly deferred rules
- [ ] Fail with a direct list of missing `rule_id@dialect` targets

**Acceptance:**
- a single test produces the current support-surface coverage verdict
- missing coverage is reported as exact `rule_id@dialect` rows
- the gate reflects the real extractor/rule support surface, not theoretical policy presence

---

## Task 3: Populate corpus cases for supported rule surfaces

**Files:**
- `testdata/sql-corpus/mysql/**`
- `testdata/sql-corpus/tidb/**`
- `testdata/sql-corpus/postgresql/**`

- [ ] Add offline DDL corpus probes for naming, PK, and broad rule families where supported
- [ ] Add offline DML corpus probes for supported DML rule families
- [ ] Add metadata-aware corpus cases for existence / denylist / row-count / index-owner paths
- [ ] Add PostgreSQL probes only for surfaces currently supported by the PostgreSQL extractor/rule contract
- [ ] Remove or avoid PostgreSQL corpus expectations that depend on unsupported extractor facts

**Acceptance:**
- every supported target in the matrix has at least one corpus case
- PostgreSQL does not over-claim coverage for unsupported fact surfaces
- corpus cases remain readable and single-purpose enough to debug

---

## Task 4: Integrate the gate into local and CI verification

**Files:**
- `Makefile`
- `.github/workflows/release-smoke.yml`

- [ ] Add a reusable local target such as `make sql-corpus-gates`
- [ ] Route that target through `release-test-gates`
- [ ] Add an explicit release-smoke workflow step for the gate

**Acceptance:**
- developers can run one local command for corpus-contract verification
- release smoke and release test paths both execute the coverage gate
- support-surface drift becomes a blocking CI failure

---

## Task 5: Document the contract

**Files:**
- `docs/dev/testing.md`
- `docs/plans/2026-04-18-sql-corpus-supported-rule-coverage-design.md`
- `docs/plans/2026-04-18-sql-corpus-supported-rule-coverage-implementation.md`

- [ ] Document the `make sql-corpus-gates` command
- [ ] Explain that corpus coverage means supported `rule_id × dialect` coverage
- [ ] Explain that this is narrower than “all policy keys on all dialects”
- [ ] Record PostgreSQL extractor-boundary deferrals clearly

**Acceptance:**
- a new contributor can understand what “100% corpus coverage” means in this repository
- the docs do not conflate rule-surface coverage with Go line coverage

---

## Verification

Run at minimum:

```bash
make sql-corpus-gates
go test ./internal/application/audit -tags postgresql -count=1
make release-test-gates
git diff --check
```

Optional broader verification:

```bash
go test ./... -count=1
```

---

## Completion Criteria

The implementation is complete when:

- the harness accepts inline config and metadata-backed fixtures
- the supported-rule coverage gate passes
- corpus fixtures cover every current supported `rule_id × dialect` target
- CI executes the gate automatically
- docs explain the contract without over-claiming support
