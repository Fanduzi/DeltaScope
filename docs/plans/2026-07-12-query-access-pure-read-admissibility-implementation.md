# Implementation Plan: Query Access Pure-Read Admissibility

Date: 2026-07-12
Branch: `query-access-pure-read-admissibility`
Depends on: design + decision (Proposed) from Task 1
Rule: one focused commit per task; run listed gates before moving on
Do not push/tag/release unless explicitly instructed later

## Global non-goals (all tasks)

- MCP query-access tool
- Grant evaluation / authn / RLS / masking / SQL rewrite
- View definition expansion
- Dynamic SQL
- Name-only trust roots
- Version bump / release notes / npm / Homebrew (unless a later release task)
- Audit rule engine changes
- `severity` field

## Global safety rules

- No promote without OID-bound trusted identity.
- Fail closed on missing resolver, unknown, multi-match, error, coercion gap.
- No raw SQL / credentials / connection strings in public results or reason text.
- Do not regress MySQL admissible corpus cases.
- Decision record stays `Proposed` until implementation evidence exists; flip to
  `Accepted` only in a dedicated docs task after gates pass.

## GitNexus (implementation tasks)

Before editing symbols in later tasks:

- `gitnexus_impact` on targets listed per task (upstream).
- `gitnexus_detect_changes` before each commit.
- Re-analyze after commits if required by repo hooks.

Task 1 (this design) is docs-only: detect_changes only.

---

## Task 1 — Design & plan (this commit)

**Status:** current
**Deliverables:**

- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (Proposed)
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md`
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md`
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-omo-prompts.md`

**File scope:** docs only
**Gates:**

```bash
make decision-record-gate
git diff --check
# sanity: no severity, no MCP-supported claims, no credential-leak promises
# detect_changes: docs only
git commit  # message: docs: design query access pure read admissibility
```

**Stop if:** design cannot define identity proof without name allowlists.

---

## Task 2 — Characterization tests for current PG admission freeze

**Goal:** lock today's behavior so refactors cannot silently change MySQL or
hide the PG hard-stop without intent.

**File scope (tests only preferred):**

- `internal/application/queryaccess/*_test.go`
- optional parser tests under `internal/infrastructure/parser/postgresql/`

**GitNexus targets:** `reclassifyAfterResolution`, `recomputeAdmission`,
`AnalyzePostgreSQL`, `ExtractTiDBQueryAccess` (impact before edit if production
touched — prefer tests only).

**Work:**

- Assert PG `WHERE id = 1` → classification indeterminate, admission
  indeterminate without effect resolver.
- Assert MySQL equivalent remains admissible (regression).
- Assert `reclassifyAfterResolution` currently never lifts PG.

**Gates:**

```bash
go test ./internal/application/queryaccess/ -count=1
go test -tags postgresql ./internal/application/queryaccess/ -count=1
git diff --check
```

**Commit:** `test: characterize query access pg admission freeze`
**Stop if:** characterization contradicts design assumptions — update design
before coding.

---

## Task 3 — Domain reason codes + result validation for unproven effects

**Goal:** additive bounded reason codes for unproven identity.

**File scope:**

- `internal/domain/queryaccess/model.go`
- `internal/domain/queryaccess/normalize.go` (+ tests)
- `pkg/deltascope` only if public reason enums are exported (prefer keep
  internal until needed)

**New codes (names finalizable in task):**

- `unproven_operator_effect`
- `unproven_function_effect`
- `unproven_cast_effect`
- `identity_lookup_failed`

**GitNexus targets:** `ReasonCode`, `ValidateResult`

**Gates:**

```bash
go test ./internal/domain/queryaccess/ -count=1
go test ./pkg/deltascope/ -run QueryAccess -count=1
```

**Commit:** `feat: add query access unproven effect reason codes`
**Stop if:** public JSON shape change requires premature versioning debate —
keep codes additive `omitempty`.

---

## Task 4 — Extract PostgreSQL effect candidates (no trust yet)

**Goal:** replace boolean "has operator/function" with structured candidates
while **keeping classification indeterminate** for those candidates until
Task 6–7 prove identity.

**File scope:**

- `internal/infrastructure/parser/postgresql/query_access.go`
- PG parser tests
- `internal/application/queryaccess/extract_postgresql.go` mapping

**Work:**

- Walk `A_Expr` / `FuncCall` / `TypeCast`; record `NamePath`, arg structure.
- Still classify as indeterminate when any unproven candidate exists (default).
- Do not introduce name allowlists.

**GitNexus targets:** `ExtractQueryAccess`, `classifyStatement`,
`pgContainsOperatorExpr`, `AnalyzePostgreSQL`

**Gates:**

```bash
go test -tags postgresql ./internal/infrastructure/parser/postgresql/ -run QueryAccess -count=1
go test -tags postgresql ./internal/application/queryaccess/ -count=1
```

**Commit:** `feat: extract postgresql query access effect candidates`
**Stop if:** `pg_query` cannot expose operator name tokens reliably.

---

## Task 5 — Extend relation metadata with type OIDs + EffectIdentityResolver API

**Goal:** types for operands + resolver interface.

**File scope:**

- `internal/application/queryaccess/contracts.go`
- `internal/infrastructure/metadata/postgresql/query_access_resolver.go`
- `internal/infrastructure/metadata/mysql/query_access_resolver.go` (columns
  may stay without OID if MySQL phase-1 untouched; keep compile parity)
- `pkg/deltascope/query_access.go` public optional resolver surface
- README package docs if exports change

**Work:**

- Add `TypeOID`/`TypeName` to column schema where available (PG).
- Define `EffectIdentityResolver` methods + identity structs.
- Fake resolvers for unit tests.

**GitNexus targets:** `SchemaResolver`, `QueryAccessRequest`,
`QueryAccessResolver`, `AnalyzeQueryAccess`

**Gates:**

```bash
go test ./internal/application/queryaccess/ -count=1
go test -tags postgresql ./internal/infrastructure/metadata/postgresql/ -count=1
go test ./pkg/deltascope/ -run QueryAccess -count=1
```

**Commit:** `feat: add query access effect identity resolver contract`
**Stop if:** public API export set balloons — keep proof engine internal and
only expose a minimal interface.

---

## Task 6 — PostgreSQL catalog identity implementation (pg_operator/pg_proc/pg_cast)

**Goal:** real proof against `pg_catalog` with exact type match; trust policy.

**File scope:**

- `internal/infrastructure/metadata/postgresql/` new identity resolver file(s)
- integration tests with sqlmock or existing test DB patterns

**Work:**

- Implement ResolveOperator/Function/Cast as designed.
- Trust only `pg_catalog` + volatility `i|s`.
- Unknown/error → typed unknown/error for application mapping.
- Scrub secrets from errors.

**GitNexus targets:** new symbols under postgresqlmeta; impact on openers if any

**Gates:**

```bash
go test -tags postgresql ./internal/infrastructure/metadata/postgresql/ -count=1
# integration if available under existing patterns
```

**Commit:** `feat: resolve postgresql effect identities from pg_catalog`
**Stop if:** unique matches require full coercion graph for the phase-1 matrix
→ return to design / kill criteria.

---

## Task 7 — Application proof engine + admission recompute fix

**Goal:** wire candidates + identity into classification/admission; remove PG
hard-stop; fix sticky indeterminate admission when proof succeeds.

**File scope:**

- `internal/application/queryaccess/service.go`
- related tests
- extract adapters if needed

**Work:**

- Implement decision order from design §4.3.
- Remove unconditional PG ban in `reclassifyAfterResolution`.
- Ensure MySQL path unchanged when no effect resolver.
- Map lookup failures to new reason codes; never `admissible`.

**GitNexus targets:** `Service.Analyze`, `recomputeAdmission`,
`reclassifyAfterResolution`

**Gates:**

```bash
go test ./internal/application/queryaccess/ -count=1
go test -tags postgresql ./internal/application/queryaccess/ -count=1
go test ./pkg/deltascope/ -run QueryAccess -count=1
go test ./internal/interfaces/cli/ -run QueryAccess -count=1
go test ./internal/interfaces/http/ -run QueryAccess -count=1
go test ./internal/interfaces/mcp/ -run 'TestNewServerExposesCoreTools' -count=1
```

**Commit:** `feat: prove trusted effects for query access admission`
**Stop if:** any test requires name-only trust to pass.

---

## Task 8 — Corpus expansion (positives + adversarial)

**Goal:** encode support matrix in `testdata/query-access/`.

**File scope:**

- `testdata/query-access/postgresql/*` new cases
- corpus runner may need optional fake identity resolver fixtures
- `testdata/query-access/mysql/*` regression only

**Work:**

- Add all design §8 cases.
- PG corpus runner (`-tags postgresql`) must exercise proof fixtures.
- Ensure MCP still not claimed.

**Gates:**

```bash
make query-access-corpus-gates
go test -tags postgresql ./internal/application/queryaccess/ -run Corpus -count=1
```

**Commit:** `test: expand query access pure read corpus`
**Stop if:** fixtures cannot express identity without embedding secrets.

---

## Task 9 — Docs + decision Accepted + recipe correction

**Goal:** user-facing reference/recipe accuracy; accept decision.

**File scope:**

- `docs/reference/query-access-analysis.md` (+ zh)
- `docs/recipe/query-platform-access-analysis.md` (+ zh)
- decision status → Accepted + verification evidence filled
- package README if exports changed

**Work:**

- Replace "PostgreSQL admission is always indeterminate".
- Document resolver obligations and fail-closed cases.
- No severity; no MCP tool claim.

**Gates:**

```bash
make decision-record-gate
# docs example gates if release surfaces touched
git diff --check
```

**Commit:** `docs: document proven pure-read query access admissibility`

---

## Task 10 — Optional hardening / kill review

**Goal:** final self-grill before any release discussion.

**Checklist:**

- [ ] No name-only trust path in production code
- [ ] MySQL regressions green
- [ ] PG positives only with proof fixtures
- [ ] Adversarial set green
- [ ] No-leak tests green
- [ ] MCP tools still 4
- [ ] Decision Accepted with evidence
- [ ] Kill criteria not triggered

**If kill criteria hit:** commit a short `docs/decisions/` note superseding
implementation attempt as audit-only; leave code unmerged or revert feature
commits on the milestone branch.

---

## Suggested task graph

```text
T1 design
  → T2 characterize
  → T3 reason codes
  → T4 effect candidates
  → T5 resolver API + type OIDs
  → T6 catalog identity
  → T7 proof engine
  → T8 corpus
  → T9 docs accept
  → T10 review
```

No parallel tasks that edit `service.go` and extractors without integration.

## Release note (later, not this plan)

When product is ready for a versioned release (separate milestone close):

- Call out public admission distribution change for PostgreSQL.
- Emphasize proof requirements and non-goals.
- Do not invent version numbers in this plan.
