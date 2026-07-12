# OMO Prompts: Query Access Pure-Read Admissibility

Copy one task block at a time. Stay on branch
`query-access-pure-read-admissibility`. Do not push, tag, release, bump
versions, publish npm/Homebrew, or open MCP query-access tools unless a later
human instruction says so.

Read first:

- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md`
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md`
- Skills: `no-co-author`, repo `Agents.md` / GitNexus rules

Hard rules for every task:

- **No syntax-name allowlist as trust root.** Operator/function/cast promotion
  requires unique catalog-resolved identity **and** membership in the
  application-maintained, version-scoped, per-item audited trusted-effect
  manifest.
- **No generic volatility trust.** Never implement
  `pg_catalog + provolatile in (i,s) ⇒ Trusted` or any equivalent. Volatility
  and catalog membership are **facts**, not trust.
- Identity resolver returns **facts only**; trust policy lives in
  application/domain. Callers/resolvers must not claim `Trusted`.
- Fail closed: missing resolver, unknown, multi-match, error, coercion gap,
  non-manifest identity (including `pg_catalog` stable/immutable not in
  manifest), function-backed cast (phase 1 default) → `indeterminate`.
- Do not regress MySQL admissible corpus cases; do not change MySQL/TiDB
  behavior for parity.
- No `severity` field; no raw SQL / credentials / connection strings / catalog
  query text in public results.
- MCP must remain without query-access tool.
- No grant evaluation / RLS / masking / SQL rewrite.
- One focused commit per task; no amend/rebase/merge unless instructed.
- Run GitNexus `impact` before editing symbols; `detect_changes` before commit.
- If a kill criterion from the design triggers (including inability to maintain
  a bounded version-scoped manifest, or pressure to use name/volatility
  allowlists), stop and report; do not invent a weaker trust model.
- **Do not start production proof/promotion work until Task 2 (manifest
  research) completes or kills the spike.**

---

## Task 1b — Trust-policy docs (if re-running)

```text
On branch query-access-pure-read-admissibility in /Users/fan/GolangProjects/DeltaScope.

Update only the four design/decision/plan/omo docs to enforce:
- catalog OID/namespace/types/volatility are necessary facts, not Trusted
- Phase 1 trust = application-maintained, version-scoped, per-item audited effect identity manifest
- no name allowlist; no generic STABLE/IMMUTABLE allowlist
- adversarial: non-manifest pg_catalog stable, current_setting, pg_get_*, UDF stable, function-backed cast → indeterminate
- implementation plan: Task 2 = manifest research before resolver/proof engine
- kill if only name or volatility class allowlist is feasible

No production code. Commit: docs: tighten query access effect trust policy
No push.
```

---

## Task 2 — Manifest research + version compatibility

```text
On branch query-access-pure-read-admissibility in /Users/fan/GolangProjects/DeltaScope.

Implement Task 2 from docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md.

Produce a bounded, version-scoped, per-item audited trusted-effect identity
manifest research deliverable BEFORE any production admission promotion:

1) Inventory phase-1 candidates only: closed comparison operator identities,
   BoolExpr structural treatment, COUNT aggregate feasibility, binary
   no-function casts only if auditable.
2) For each candidate, draft semantic audit proof:
   - unique data deps = extracted AST operands only
   - no relation/config/role/file/network hidden reads
   - requirements completeness preserved
3) Explicitly list always-indeterminate: non-manifest pg_catalog stable/immutable,
   current_setting, pg_get_*, user-defined stable effects, function-backed casts.
4) Version range policy and OID stability approach.
5) Kill review: if work collapses to name allowlist, volatility class allowlist,
   or unmaintainable open set → STOP as audit spike; do not implement weaker trust.

Prefer docs + characterization tests. Do not implement production Trusted
promotion. Do not change MySQL/TiDB behavior.

Gates: decision-record-gate if docs updated; go tests if tests added; git diff --check;
gitnexus detect_changes (docs/tests only expected).

Commit: docs: research query access trusted effect manifest
(or test: ... if primarily tests)
No co-authors. No push.
```

---

## Task 3 — Characterize PG admission freeze

```text
On branch query-access-pure-read-admissibility in /Users/fan/GolangProjects/DeltaScope.

Implement Task 3 from docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md.

Add characterization tests only (prefer no production code) proving:
1) PostgreSQL SELECT with WHERE equality is classification+admission indeterminate without effect identity resolver.
2) MySQL equivalent remains admissible (regression).
3) reclassifyAfterResolution currently does not lift PostgreSQL indeterminate.
4) Document intended adversarial outcomes (as tests or table-driven fixtures):
   non-manifest pg_catalog stable, current_setting, pg_get_*, function-backed cast,
   user-defined stable → must remain indeterminate when those paths are exercised.

Follow design diagnosis in docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md §2.
Task 2 manifest research must still complete before production trust code (Tasks 6–8).

Gates: go test ./internal/application/queryaccess/ -count=1; go test -tags postgresql ./internal/application/queryaccess/ -count=1; git diff --check; gitnexus detect_changes.

Commit: test: characterize query access pg admission freeze
No co-authors. No push.
```

---

## Task 4 — Unproven effect reason codes

```text
On branch query-access-pure-read-admissibility.

Implement Task 4 from the implementation plan: additive domain reason codes
unproven_operator_effect, unproven_function_effect, unproven_cast_effect,
identity_lookup_failed (optional effect_not_in_trust_manifest) with tests and
ValidateResult compatibility.

Do not change admission logic yet. No severity field.
Do not implement name or volatility allowlists.

GitNexus impact on ReasonCode / ValidateResult before edits.
Commit: feat: add query access unproven effect reason codes
No push.
```

---

## Task 5 — Extract PG effect candidates

```text
On branch query-access-pure-read-admissibility.

Implement Task 5: PostgreSQL extractor emits structured effect candidates
(A_Expr / FuncCall / TypeCast name paths + arg structure). Classification must
remain indeterminate for unproven effects. Do NOT add name allowlists. Do NOT
add volatility-based trust. Do NOT promote admission.

Map candidates through extract_postgresql.go without trusting names.

Gates: go test -tags postgresql on parser + application queryaccess packages.
Commit: feat: extract postgresql query access effect candidates
No push.
```

---

## Task 6 — Resolver contract + type OIDs (facts only)

```text
On branch query-access-pure-read-admissibility.

Implement Task 6: extend SchemaResolver column metadata with type OIDs for
PostgreSQL; add EffectIdentityResolver application contract and public SDK
optional surface if needed; fake resolvers for tests.

CRITICAL: identity structs return FACTS only (OID, namespace, types, volatility,
cast method). Do NOT include a caller-settable Trusted field. Document that
trust is application/domain manifest policy.

Do not implement live pg_operator queries yet (Task 7).
Do not implement TrustPolicy promotion yet (Task 8).
Do not change MCP tools. Do not change MySQL/TiDB behavior.

Commit: feat: add query access effect identity resolver contract
No push.
```

---

## Task 7 — pg_catalog identity implementation (facts only)

```text
On branch query-access-pure-read-admissibility.

Implement Task 7: PostgreSQL EffectIdentityResolver against pg_catalog
(pg_operator / pg_proc / pg_cast) with exact type match. Return facts only:
OID, namespace, volatility, castfunc/castmethod. Scrub secrets from errors.
Unknown/multi-match → not a unique identity.

FORBIDDEN: "trust only pg_catalog + volatility i|s" or any equivalent Trusted
assertion in this layer. Trust is Task 8 + Task 2 manifest.

If unique identity requires a full coercion planner for the phase-1 matrix,
STOP and report kill criterion instead of adding name or volatility allowlists.

Commit: feat: resolve postgresql effect identities from pg_catalog
No push.
```

---

## Task 8 — Trust policy + proof engine + admission recompute

```text
On branch query-access-pure-read-admissibility.

Implement Task 8 from the plan and design decision order:
- ONLY after Task 2 manifest research is complete and not killed
- TrustPolicy.IsTrusted(identity, manifest, version) is the sole Trusted path
- Wire effect candidates + identity facts into Service.Analyze
- Remove unconditional PostgreSQL ban in reclassifyAfterResolution
- Fix sticky indeterminate admission when classification becomes read_only after
  manifest-proven effects
- Relation-only resolver must NOT admit effect-bearing PG SQL without effect proof
- Resolved pg_catalog + stable/immutable NOT in manifest → indeterminate
- current_setting / pg_get_* / UDF stable / function-backed cast → indeterminate
- MySQL path without effect resolver must remain behavior-compatible
- FORBIDDEN: name allowlist trust; universal volatility i|s trust

Gates: application/queryaccess tests (with and without postgresql tag),
pkg/deltascope QueryAccess, CLI/HTTP QueryAccess, MCP TestNewServerExposesCoreTools.

Commit: feat: prove trusted effects for query access admission
No push.
```

---

## Task 9 — Corpus expansion

```text
On branch query-access-pure-read-admissibility.

Implement Task 9: expand testdata/query-access per design §8:
- safe positives only with proof fixtures AND manifest hits
- adversarial: non-manifest pg_catalog stable, current_setting, pg_get_*,
  user-defined stable, function-backed cast, no resolver, resolver error,
  unknown OID, multi-match, coercion gap, ambiguity, wildcard
- requirements completeness: admissible results must not omit known physical
  source columns
- no-leak: no SQL, literals, credentials, connection strings, catalog query text;
  no severity field
Keep MySQL regressions green.

make query-access-corpus-gates must pass.

Commit: test: expand query access pure read corpus
No push.
```

---

## Task 10 — Docs + accept decision

```text
On branch query-access-pure-read-admissibility.

Implement Task 10: update query-access reference + recipe (EN/ZH) to replace
"PostgreSQL admission is always indeterminate" with proof+manifest-gated wording.
State clearly that STABLE/IMMUTABLE catalog membership alone never admits.
Fill decision verification evidence; set status Accepted only if gates green.
No severity; no MCP query-access claims; no version bump/release.

Commit: docs: document proven pure-read query access admissibility
No push.
```

---

## Task 11 — Final review (human or agent)

```text
On branch query-access-pure-read-admissibility.

Run Task 11 checklist from the implementation plan. Confirm kill criteria did
not force a name-only or volatility-class design. Confirm Task 2 produced a
bounded version-scoped manifest (or milestone killed as audit-only).

Report: corpus counts, sample admissible PG SQL (if any), adversarial cases
including non-manifest catalog stable, MCP tool list still four, decision
status, whether ready for a future release task (do not release).
```
