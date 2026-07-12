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
- Generic volatility trust roots (`pg_catalog` + `i|s` as universal Trusted)
- Version bump / release notes / npm / Homebrew (unless a later release task)
- Audit rule engine changes
- `severity` field
- Changing MySQL/TiDB current admission behavior

## Global safety rules

- Catalog identity resolution returns **facts only** (OID, namespace, types,
  volatility). It does **not** assert Trusted.
- Trust is decided only by application/domain policy against a
  **version-scoped, per-item audited effect identity manifest**.
- No promote without unique resolved identity **and** manifest membership.
- Fail closed on: missing resolver, resolver error, unknown OID, multi-match,
  coercion gap, non-manifest identity (including `pg_catalog` + stable/immutable
  not in manifest), function-backed cast (phase 1 default).
- No raw SQL / credentials / connection strings / catalog query text in public
  results or reason text.
- Do not regress MySQL admissible corpus cases.
- Decision record stays `Proposed` until implementation evidence exists; flip to
  `Accepted` only in a dedicated docs task after gates pass.
- **Never** implement "trust only `pg_catalog` + volatility `i|s`" or any
  equivalent universal predicate.

## GitNexus (implementation tasks)

Before editing symbols in later tasks:

- `gitnexus_impact` on targets listed per task (upstream).
- `gitnexus_detect_changes` before each commit.
- Re-analyze after commits if required by repo hooks.

Task 1 (design) and Task 1b (trust-policy doc fix) are docs-only:
detect_changes only.

---

## Task 1 — Design & plan (done)

**Status:** done (`docs: design query access pure read admissibility`)
**Deliverables:**

- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (Proposed)
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md`
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md`
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-omo-prompts.md`

**Note:** Initial design incorrectly treated `pg_catalog` + volatility `i|s`
as a general trust predicate. Corrected in Task 1b before any production code.

---

## Task 1b — Trust-policy tightening (docs only)

**Status:** done (`docs: tighten query access effect trust policy`)

---

## Task 2 — Manifest research + version compatibility study

**Status:** done (this commit) — **feasibility: Proceed**
**Goal:** answer whether a bounded, version-scoped, per-item audited PostgreSQL
effect identity manifest is maintainable — without implementing resolver/trust
code.

**Deliverables (docs only):**

- Decision record T2 Evidence + **Proceed** conclusion
- Design Appendix A: version matrix, identity tuples, candidate/rejected ledger,
  privilege/no-leak, volatility-as-fact
- This plan: later tasks gated on T2 ledger
- OMO prompts: subsequent tasks must read and obey the ledger

**T2 conclusions (normative for later tasks):**

- **Proceed** with closed phase-1 set:
  - structural `BoolExpr` AND/OR/NOT
  - 54 same-type comparison operators on
    `{bool,int2,int4,int8,float4,float8,numeric,text,oid}` ×
    `{=,<>,<,>,<=,>=}`
  - `count(*)` OID 2803 and `count(anyelement)` OID 2147
- **PostgreSQL major claim for effect identity:** **14–17** (probed on 14/16/17);
  CI primary **17**; **do not claim 12/13/15 without re-probe**; **18** out
  until parser + re-probe
- Casts: phase-1 **default omit** (binary cast row OIDs unstable; function-backed
  casts rejected)
- Volatility `i|s` is fact only; never Trusted
- Rejected: `current_setting`, `pg_get_*`, UDF effects, non-manifest catalog
  effects, function-backed casts, name allowlists

**File scope:** docs only (four milestone files). Ephemeral `/tmp` probes not
committed. **No** `internal/**`, `pkg/**`, `cmd/**`, Makefile, corpus, or
version-surface edits.

**Gates:**

```bash
make decision-record-gate
git diff --check
# sanity: no "pg_catalog + volatility ⇒ Trusted"; no name allowlist trust;
#         no MCP-supported; no severity; no credential/SQL leak promises
# detect_changes: docs only; no execution flows
git commit  # message: docs: research query access effect identity manifest
```

**Stop if (did not fire):** kill criteria 4–6. If later tasks violate the
ledger, stop and re-open T2 rather than widen trust.

---

## Global implementation precondition (Tasks 3–11)

All production trust/resolver/proof/corpus work **must**:

1. Read design Appendix A + decision T2 Evidence first.
2. Implement **only** the closed candidate ledger (or a strict subset).
3. Fail closed outside the ledger and outside majors 14–17 (until re-probe).
4. Never introduce name or volatility-class allowlists.
5. Treat T2 **Proceed** as permission to continue research→implementation
   sequencing — **not** as Accepted product behavior or a release green light.

If implementation discovers the ledger is wrong, update docs and re-audit;
do not silently expand Trusted.

---

## Task 3 — Characterization tests for current PG admission freeze

**Status:** done (`test: characterize query access effect identity candidates`)
**Precondition:** T2 Proceed ledger (Appendix A) is authoritative for intended
future adversarial outcomes.
**Goal:** lock today's behavior so refactors cannot silently change MySQL or
hide the PG hard-stop without intent. Extend adversarial expectations that
future code must not promote on volatility alone.

**File scope (tests + corpus + design evidence only):**

- `internal/application/queryaccess/effect_identity_characterization_postgresql_tag_test.go`
- `internal/application/queryaccess/effect_identity_mysql_tidb_regression_test.go`
- `internal/infrastructure/parser/postgresql/query_access_effect_identity_postgresql_tag_test.go`
- `testdata/query-access/postgresql/*` (new fixtures)
- Decision record T3 Evidence + this plan + design Appendix A.8

**GitNexus targets:** new test functions only (no production symbol edits).
`detect_changes` before commit. Impact not required for new tests; required if
any existing helper/production symbol is edited.

**Work completed:**

- Structural `AND`/`OR`/`NOT`: BoolExpr is structural, not catalog identity proof.
- T2 candidate comparison matrix representatives (`=`,`<>`,`<`,`>`,`<=`,`>=`)
  stay classification+admission `indeterminate` (with/without relation resolver).
- `COUNT(*)` / `COUNT(column)` stay indeterminate.
- Type-resolution / coercion-looking comparisons stay indeterminate.
- Rejected ledger (`current_setting`, `set_config`, `pg_get_*`, file readers,
  UDF-looking / non-`pg_catalog` schema effects, function-backed cast) stay
  indeterminate.
- Unknown/coercion/multi-match shapes, no resolver, resolver error, incomplete
  metadata stay indeterminate; no-leak / no-severity assertions.
- `reclassifyAfterResolution` never lifts PostgreSQL; MySQL operator-bearing
  SELECT remains admissible (TiDB same path).

**T3 Evidence notes (normative):**

- Characterization ≠ support. Candidate ledger entries are **not** Trusted.
- PostgreSQL promotion remains **forbidden** until manifest + identity resolver
  + proof engine land in later tasks.
- Rejected ledger continues fail-closed.

**Gates:**

```bash
go test -tags postgresql ./internal/infrastructure/parser/postgresql/... -count=1
go test -tags postgresql ./internal/application/queryaccess/... -count=1
go test ./internal/application/queryaccess/... -count=1
make query-access-corpus-gates
go test ./... -count=1
go test -tags postgresql ./... -count=1
make decision-record-gate
git diff --check
go mod tidy && git diff --exit-code go.mod go.sum
golangci-lint run ./...
```

**Commit:** `test: characterize query access effect identity candidates`
**Stop if:** characterization contradicts design assumptions — update design
before coding.

**Ordering note:** Task 2 complete before T3. Production identity trust
implementation remains Tasks 4+ (reason codes) and 5–8 (candidates → resolver →
manifest → proof).

---

## Task 4 — Domain reason codes + result validation for unproven effects

**Goal:** additive bounded reason codes for unproven identity / non-manifest
trust denial.

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
- optional: `effect_not_in_trust_manifest` (if product wants distinction from
  unproven; else map non-manifest to unproven_*)

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

## Task 5 — Extract PostgreSQL effect candidates (no trust yet)

**Goal:** replace boolean "has operator/function" with structured candidates
while **keeping classification indeterminate** for those candidates until later
tasks prove identity **and** apply the manifest.

**File scope:**

- `internal/infrastructure/parser/postgresql/query_access.go`
- PG parser tests
- `internal/application/queryaccess/extract_postgresql.go` mapping

**Work:**

- Walk `A_Expr` / `FuncCall` / `TypeCast`; record `NamePath`, arg structure.
- Still classify as indeterminate when any unproven candidate exists (default).
- Do not introduce name allowlists.
- Do not introduce volatility-based trust.

**GitNexus targets:** `ExtractQueryAccess`, `classifyStatement`,
`pgContainsOperatorExpr`, `AnalyzePostgreSQL`

**Gates:**

```bash
go test -tags postgresql ./internal/infrastructure/parser/postgresql/ -run QueryAccess -count=1
go test -tags postgresql ./internal/application/queryaccess/ -count=1
```

**Commit:** `feat: extract postgresql query access effect candidates`
**Stop if:** `pg_query` cannot expose operator name tokens reliably.

### T5 Evidence (completed)

- Implemented internal `EffectCandidate` collection on the complete SelectStmt
  effect traversal (same walk as unproven reasons).
- Candidates include kind, ordinal, name path, explicit schema, arity, operand
  kind hints, aggregate/window/filter flags, cast target type path.
- **Not** a trust root: no Trusted bit; no catalog resolve; no admission lift.
- Public surfaces still expose only domain `Result` (bounded `unproven_*`
  reasons); candidates stay parser/application-internal.
- Structural BoolExpr does not emit candidates.
- MySQL/TiDB not modified.
- Decision status remains **Proposed**.

---

## Task 6 — Effect identity resolver contract (facts only) — DONE

**Precondition:** T2 ledger; facts-only API; no Trusted field.
**Goal:** typed, fail-closed **facts-only** resolver contract for T7/T8.
**Status:** done — contract only; **no** pg_catalog adapter, **no** manifest
trust, **no** Analyze invocation / admission change, **no** public SDK field.

**File scope (actual):**

- `internal/domain/queryaccess/model.go` + `normalize.go` (`IdentityStatus`)
- `internal/application/queryaccess/identity_resolver.go` (+ tests)
- `internal/application/queryaccess/contracts.go` (`ColumnSchema.TypeOID` only)
- package READMEs + decision T6 Evidence
- **Not** in T6: PostgreSQL catalog SQL, public `pkg/deltascope.QueryAccessRequest`
  resolver field, `Service.Analyze` wiring

**Work delivered:**

- Batch `EffectIdentityResolver.ResolveEffectIdentities` returning facts only.
- Bounded statuses aligned with `IdentityFailure` style (`resolved` +
  `unknown` / `ambiguous` / `coercion_gap` / `lookup_failed` / `unavailable`).
- Ordinal uniqueness, deterministic sort, partial-failure completion, cancel
  semantics, free-text → `lookup_failed` sanitization.
- Optional `ColumnSchema.TypeOID` (unpopulated by catalog in T6).
- Fake resolver in tests; public JSON / Analyze freeze tests.

**T6 vs later:**

| Task | Responsibility |
|------|----------------|
| **T6 (this)** | Facts-only contract + helpers + tests |
| **T7** | PostgreSQL `pg_catalog` adapter implementing the contract |
| **T8** | Manifest trust + proof engine; may discuss Analyze wiring / public injection |

**Commit:** `feat: define query access identity resolver contract`

---

## Task 7 — PostgreSQL catalog identity implementation (facts only)

**Precondition:** T2 ledger. Resolve operators/functions needed for the closed
candidate set with exact type match; do not build open-ended catalogs of trust.
**Goal:** real identity resolution against `pg_catalog` with exact type match;
return OIDs, namespace, volatility, cast method as **facts**. Do **not** apply
"pg_catalog + i|s ⇒ trusted".

**File scope:**

- `internal/infrastructure/metadata/postgresql/` new identity resolver file(s)
- integration tests with sqlmock or existing test DB patterns

**Work:**

- Implement ResolveOperator/Function/Cast as designed (facts).
- Unique match required; zero rows / multi-match → unknown/error for application.
- Function-backed cast facts must be distinguishable (castfunc / castmethod).
- Scrub secrets from errors.
- **Do not** set Trusted in this layer.

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

## Task 8 — Trust policy + application proof engine + admission recompute fix

**Precondition:** T2 **Proceed** ledger is encoded as the only Trusted set
(closed operators + count aggregates + structural bool; majors 14–17; casts
omitted unless a later audited expansion).
**Goal:** load T2 audited manifest into application/domain trust policy;
wire candidates + identity facts into classification/admission; remove PG
hard-stop; fix sticky indeterminate admission when proof succeeds.

**File scope:**

- `internal/application/queryaccess/service.go` (+ trust policy module)
- optional `internal/domain/queryaccess/` for manifest types if domain-owned
- related tests
- extract adapters if needed

**Work:**

- Implement decision order from design §4.5.
- `TrustPolicy.IsTrusted(identity, manifest, version)` — only path to Trusted.
- Manifest entries only for Task 2 audited identities; everything else
  indeterminate (including resolved `pg_catalog` + stable not in manifest).
- Remove unconditional PG ban in `reclassifyAfterResolution`.
- Ensure MySQL path unchanged when no effect resolver.
- Map lookup failures and non-manifest denials to reason codes; never
  `admissible`.
- Reject function-backed casts by default (indeterminate).
- **Do not** implement universal volatility allowlist.

**GitNexus targets:** `Service.Analyze`, `recomputeAdmission`,
`reclassifyAfterResolution`, new trust-policy symbols

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
**Stop if:** any test requires name-only or volatility-class trust to pass.

---

## Task 9 — Corpus expansion (positives + adversarial)

**Goal:** encode support matrix and adversarial matrix in
`testdata/query-access/`.

**File scope:**

- `testdata/query-access/postgresql/*` new cases
- corpus runner may need optional fake identity resolver fixtures + manifest
  fixtures
- `testdata/query-access/mysql/*` regression only

**Work:**

- Add all design §8 cases, including:
  - non-manifest `pg_catalog` + stable → indeterminate
  - `current_setting` / `pg_get_*` → indeterminate
  - user-defined stable → indeterminate
  - function-backed cast → indeterminate
  - single manifest effect promotes only with full relation/column metadata
  - strict requirements completeness for known physical sources
  - no-leak (no SQL, literals, credentials, connection strings, catalog query
    text; no `severity`)
- PG corpus runner (`-tags postgresql`) must exercise proof + manifest fixtures.
- Ensure MCP still not claimed.

**Gates:**

```bash
make query-access-corpus-gates
go test -tags postgresql ./internal/application/queryaccess/ -run Corpus -count=1
```

**Commit:** `test: expand query access pure read corpus`
**Stop if:** fixtures cannot express identity without embedding secrets.

---

## Task 10 — Docs + decision Accepted + recipe correction

**Goal:** user-facing reference/recipe accuracy; accept decision.

**File scope:**

- `docs/reference/query-access-analysis.md` (+ zh)
- `docs/recipe/query-platform-access-analysis.md` (+ zh)
- decision status → Accepted + verification evidence filled
- package README if exports changed

**Work:**

- Replace "PostgreSQL admission is always indeterminate".
- Document: resolver returns facts; trust is manifest-gated; volatility alone
  never promotes.
- Document resolver obligations and fail-closed cases.
- No severity; no MCP tool claim; no MySQL/TiDB behavior change claims beyond
  regression preservation.

**Gates:**

```bash
make decision-record-gate
# docs example gates if release surfaces touched
git diff --check
```

**Commit:** `docs: document proven pure-read query access admissibility`

---

## Task 11 — Optional hardening / kill review

**Goal:** final self-grill before any release discussion.

**Checklist:**

- [ ] No name-only trust path in production code
- [ ] No universal `pg_catalog` + volatility `i|s` trust path
- [ ] Manifest is bounded, version-scoped, per-item audited
- [ ] MySQL regressions green
- [ ] PG positives only with proof + manifest fixtures
- [ ] Adversarial set green (non-manifest catalog stable, current_setting,
      pg_get_*, UDF stable, function-backed cast, resolver failures)
- [ ] Requirements completeness fixtures green
- [ ] No-leak tests green (no SQL/creds/conn/catalog text; no severity)
- [ ] MCP tools still 4
- [ ] Decision Accepted with evidence
- [ ] Kill criteria not triggered

**If kill criteria hit:** commit a short `docs/decisions/` note superseding
implementation attempt as audit-only; leave code unmerged or revert feature
commits on the milestone branch.

---

## Suggested task graph

```text
T1 design (done)
  → T1b trust-policy docs fix (done)
  → T2 manifest research + version study (done, Proceed)
  → T3 characterize PG freeze + effect identity candidates (done)
  → T4 reason codes (done)
  → T5 effect candidates (no trust) (done)
  → T6 resolver contract (facts only) (done)
  → T7 catalog identity implementation (facts only)   ← next
  → T8 trust policy + proof engine (manifest = T2 ledger only)
  → T9 corpus
  → T10 docs accept
  → T11 review
```

No parallel tasks that edit `service.go` and extractors without integration.
**T2 Proceed is a hard precondition for T6–T8 production promotion. The T2
candidate/rejected ledger is the only allowed Trusted set unless a new
decision expands it with re-probe evidence.**

## Release note (later, not this plan)

When product is ready for a versioned release (separate milestone close):

- Call out public admission distribution change for PostgreSQL.
- Emphasize proof requirements, manifest trust, and non-goals.
- Explicitly state that STABLE/IMMUTABLE catalog membership alone never
  admits.
- Do not invent version numbers in this plan.
