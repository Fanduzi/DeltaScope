# Explainable Audit Results Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** add a shared explanation layer to DeltaScope audit findings so CLI, HTTP, and library consumers receive more actionable and self-explaining results without changing verdict semantics.

**Architecture:** keep the existing offline-first audit engine and verdict flow intact. Add a stable explanation model linked by `rule_id`, expand the rule catalog into the explanation source of truth, enrich findings after rule evaluation, and expose the same structured data through CLI, HTTP, and `pkg/deltascope`.

**Tech Stack:** Go, existing application/domain/infrastructure layers, Cobra CLI, HTTP adapter, `pkg/deltascope`, Markdown docs, Go testing

---

### Task 1: Save the milestone planning artifacts

**Files:**
- Create: `docs/plans/2026-03-24-explainable-audit-results-design.md`
- Create: `docs/plans/2026-03-24-explainable-audit-results-implementation.md`
- Create: `docs/plans/2026-03-24-explainable-audit-results-task-prompts.md`

**Step 1:** save the approved design document
**Step 2:** save the implementation plan and task prompts
**Step 3:** review naming and scope consistency across all three documents
**Step 4:** commit

### Task 2: Add the explanation model to the shared result path

**Files:**
- Modify: `internal/domain/rule/...`
- Modify: `internal/domain/report/...`
- Modify: `pkg/deltascope/...`
- Modify: affected module `README.md` files
- Test: result/finding-focused tests under the touched packages

**Step 1:** write failing tests for explanation data attached to findings without changing verdict behavior
**Step 2:** add a stable explanation model with the agreed fields
**Step 3:** thread the model through shared result types used by the audit pipeline and public package
**Step 4:** keep existing result semantics unchanged when explanation fields are absent
**Step 5:** run focused tests
**Step 6:** commit

### Task 3: Expand the rule catalog into the explanation source of truth

**Files:**
- Modify: `internal/domain/rule/catalog/...`
- Modify: rule-catalog-related README files
- Test: `internal/domain/rule/catalog/...`

**Step 1:** write failing tests for explanation metadata completeness, lookup stability, and graceful fallback
**Step 2:** extend catalog entries with summary, risk, remediation, config hints, and metadata-aware notes
**Step 3:** add the highest-value examples only where they improve remediation clarity
**Step 4:** keep execution logic separate from catalog metadata, linked only by `rule_id`
**Step 5:** run focused tests
**Step 6:** commit

### Task 4: Add post-evaluation explanation enrichment

**Files:**
- Modify: `internal/application/audit/...`
- Modify: any shared helper files needed for finding enrichment
- Modify: affected module `README.md` files
- Test: audit-service-focused tests

**Step 1:** write failing tests for enrichment after evaluation
**Step 2:** implement a shared explanation-enrichment stage that looks up catalog data by `rule_id`
**Step 3:** populate explanation fields using finding data, statement kind, and metadata-availability context
**Step 4:** ensure missing catalog data degrades to a minimal explanation instead of failing the audit
**Step 5:** verify verdicts and finding counts remain unchanged
**Step 6:** run focused tests
**Step 7:** commit

### Task 5: Expose structured explanations through `pkg/deltascope`

**Files:**
- Modify: `pkg/deltascope/audit.go`
- Modify: `pkg/deltascope/README.md`
- Test: `pkg/deltascope/...`

**Step 1:** write failing public-package tests for explanation fields in the returned result
**Step 2:** expose the explanation structure through the stable public result types
**Step 3:** document the new fields in the package README with a short example
**Step 4:** run focused tests
**Step 5:** commit

### Task 6: Add CLI explanation output without breaking compact defaults

**Files:**
- Modify: `internal/interfaces/cli/...`
- Modify: `cmd/deltascope/README.md`
- Modify: `internal/interfaces/cli/README.md`
- Test: `internal/interfaces/cli/cli_test.go`

**Step 1:** write failing CLI tests for detailed explanation output and JSON exposure
**Step 2:** add a detailed/explain-oriented output mode while preserving the current compact default output
**Step 3:** render `why`, `risk`, `suggestion`, and config hints clearly for human readers
**Step 4:** keep quiet/shell-friendly behavior predictable in non-detailed modes
**Step 5:** run focused tests
**Step 6:** commit

### Task 7: Expose structured explanations through the HTTP surface

**Files:**
- Modify: `internal/interfaces/http/...`
- Modify: `cmd/deltascope-server/README.md`
- Modify: affected HTTP README files
- Test: HTTP handler/response tests

**Step 1:** write failing HTTP tests for explanation fields in `POST /v1/audit` responses
**Step 2:** update response shaping to include structured explanation data
**Step 3:** keep existing verdict, summary, and statement semantics stable
**Step 4:** document the enriched response shape in the relevant HTTP docs
**Step 5:** run focused tests
**Step 6:** commit

### Task 8: Add metadata-awareness transparency to explanations

**Files:**
- Modify: explanation-enrichment code and any metadata-related shared types
- Modify: `internal/infrastructure/metadata/...` or shared metadata-facing helpers only if needed for clearer context propagation
- Test: metadata-aware and offline-path tests

**Step 1:** write failing tests for `metadata_note` behavior in offline and metadata-aware runs
**Step 2:** add explicit explanation notes for metadata-enhanced versus metadata-limited cases
**Step 3:** keep the offline path honest when instance facts or table snapshots are unavailable
**Step 4:** run focused tests
**Step 5:** commit

### Task 9: Update product-facing docs for explainable results

**Files:**
- Modify: `docs/reference/...`
- Modify: `docs/recipe/...`
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: affected module `README.md` files

**Step 1:** add or update docs that explain how to interpret enriched findings
**Step 2:** add CLI, HTTP, and library examples showing explanation fields in practice
**Step 3:** keep English and Chinese docs aligned where applicable
**Step 4:** update module READMEs when exported types or dependencies change
**Step 5:** run link and content sanity checks
**Step 6:** commit

### Task 10: Final verification and milestone closure

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: any changed module `README.md` files

**Step 1:** run full verification, including focused package tests, CLI tests, HTTP tests, and broader `go test ./...` coverage
**Step 2:** verify enriched explanations do not change verdict outcomes for existing cases
**Step 3:** run three-level-doc validation if required by the current repo workflow
**Step 4:** update handoff/progress/decision docs with the explanation milestone outcome
**Step 5:** commit
**Step 6:** push
