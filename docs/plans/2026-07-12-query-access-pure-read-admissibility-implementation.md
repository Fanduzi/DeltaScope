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

## Task 1b — Trust-policy tightening (docs only; current)

**Status:** current
**Goal:** eliminate P1 unsafe trust model; redefine trust as manifest-gated.

**Deliverables (update in place):**

- decision + design + this plan + omo prompts
- manifest-first task order; kill criteria for volatility/name allowlist collapse
- adversarial matrix requiring non-manifest `pg_catalog` stable → indeterminate

**File scope:** docs only (the four files above)
**Gates:**

```bash
make decision-record-gate
git diff --check
# sanity: no residual "pg_catalog + volatility i|s ⇒ Trusted" wording
# detect_changes: docs only; no execution flows
git commit  # message: docs: tighten query access effect trust policy
```

**Stop if:** design cannot define identity proof without name or volatility
class allowlists.

---

## Task 2 — Manifest research + version compatibility study

**Goal:** before any production trust or resolver wiring, produce a bounded,
version-scoped, per-item audited **trusted-effect identity manifest design**
and adversarial characterization of non-trust.

**This task must complete (or hit kill criteria) before Tasks 5–8 implement
promotion.** Prefer docs/research + characterization tests; production
admission logic stays unchanged.

**File scope (preferred):**

- Research notes committed under `docs/plans/` or an appendix in the design
  doc (e.g. phase-1 candidate inventory table)
- Characterization tests only under
  `internal/application/queryaccess/*_test.go` and/or
  `internal/infrastructure/parser/postgresql/*_test.go` if needed to lock
  current fail-closed behavior and document intended future matrix
- **No** production trust policy code that promotes admission yet

**Work:**

1. Inventory phase-1 **candidates** (not automatic trust):
   - closed set of comparison operator identities (name + left/right type OIDs)
   - whether `BoolExpr` is structural-only
   - `COUNT(*)` / `COUNT(col)` aggregate identity feasibility
   - binary (no-function) casts only if audit can prove safety
2. For each candidate, draft the audit proof:
   - unique data deps = AST operands only
   - no relation/config/role/file/network hidden reads
   - requirements completeness preserved
3. Explicitly list **must-remain-indeterminate** identities:
   - non-manifest `pg_catalog` stable/immutable
   - `current_setting`, `pg_get_*`
   - user-defined stable ops/fns/casts
   - function-backed casts
4. Version range policy: which PostgreSQL majors the manifest claims; how OID
   stability is verified across versions.
5. **Kill review:** if the inventory collapses to name allowlists, volatility
   class allowlists, or an unbounded/unmaintainable set → stop (audit spike).

**GitNexus targets:** none required if docs/tests-only; if any production symbol
is touched, impact first. Prefer zero production edits.

**Gates:**

```bash
# if tests added:
go test ./internal/application/queryaccess/ -count=1
go test -tags postgresql ./internal/application/queryaccess/ -count=1
make decision-record-gate   # if decision/design text updated
git diff --check
```

**Commit:** `docs: research query access trusted effect manifest`
(or `test:` if primarily characterization tests)

**Stop if:** kill criteria 4–6 from design (name allowlist, volatility class
allowlist, or unmaintainable manifest).

---

## Task 3 — Characterization tests for current PG admission freeze

**Goal:** lock today's behavior so refactors cannot silently change MySQL or
hide the PG hard-stop without intent. Extend adversarial expectations that
future code must not promote on volatility alone.

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
- Assert (as documentation tests / table-driven intent) that non-manifest
  `pg_catalog` stable paths, `current_setting`, `pg_get_*`, function-backed
  cast, and user-defined stable effects must remain indeterminate when those
  fixtures exist or as skipped-until-resolver placeholders.

**Gates:**

```bash
go test ./internal/application/queryaccess/ -count=1
go test -tags postgresql ./internal/application/queryaccess/ -count=1
git diff --check
```

**Commit:** `test: characterize query access pg admission freeze`
**Stop if:** characterization contradicts design assumptions — update design
before coding.

**Ordering note:** Task 2 and Task 3 may be adjacent; Task 2 (manifest research)
must not be skipped. If Task 3 is easier first for freeze-locking, still complete
Task 2 before any production identity trust implementation (Tasks 5+).

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

---

## Task 6 — Extend relation metadata with type OIDs + EffectIdentityResolver API

**Goal:** types for operands + resolver interface that returns **facts only**.

**File scope:**

- `internal/application/queryaccess/contracts.go`
- `internal/infrastructure/metadata/postgresql/query_access_resolver.go`
- `internal/infrastructure/metadata/mysql/query_access_resolver.go` (columns
  may stay without OID if MySQL phase-1 untouched; keep compile parity)
- `pkg/deltascope/query_access.go` public optional resolver surface
- README package docs if exports change

**Work:**

- Add `TypeOID`/`TypeName` to column schema where available (PG).
- Define `EffectIdentityResolver` methods + identity structs **without** a
  caller-settable `Trusted` field.
- Fake resolvers for unit tests return facts only.
- Document that trust policy lives in application/domain, not in the resolver.

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

## Task 7 — PostgreSQL catalog identity implementation (facts only)

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

**Goal:** load Task 2's audited manifest into application/domain trust policy;
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
  → T1b trust-policy docs fix (current)
  → T2 manifest research + version study   ← BEFORE any production trust
  → T3 characterize PG freeze + adversarial intent
  → T4 reason codes
  → T5 effect candidates (no trust)
  → T6 resolver API + type OIDs (facts only)
  → T7 catalog identity implementation (facts only)
  → T8 trust policy + proof engine
  → T9 corpus
  → T10 docs accept
  → T11 review
```

No parallel tasks that edit `service.go` and extractors without integration.
**Do not start T6–T8 production promotion until T2 manifest research passes
or explicitly kills the milestone as audit-only.**

## Release note (later, not this plan)

When product is ready for a versioned release (separate milestone close):

- Call out public admission distribution change for PostgreSQL.
- Emphasize proof requirements, manifest trust, and non-goals.
- Explicitly state that STABLE/IMMUTABLE catalog membership alone never
  admits.
- Do not invent version numbers in this plan.
