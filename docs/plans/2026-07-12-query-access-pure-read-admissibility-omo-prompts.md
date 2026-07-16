# OMO Prompts: Query Access Pure-Read Admissibility

Copy one task block at a time. Stay on branch
`query-access-pure-read-admissibility`. Do not push, tag, release, bump
versions, publish npm/Homebrew, or open MCP query-access tools unless a later
human instruction says so.

Read first (every task):

- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (**T2 Evidence**)
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md` (**Appendix A ledger**)
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md`
- Skills: `no-co-author`, repo `Agents.md` / GitNexus rules

Hard rules for every task:

- **Read the T2 ledger first.** Design Appendix A + decision T2 Evidence are
  normative. Phase-1 Trusted set is **only** the closed candidate ledger
  (structural BoolExpr; 54 same-type comparison ops on
  `{bool,int2,int4,int8,float4,float8,numeric,text,oid}` × `{=,<>,<,>,<=,>=}`;
  `count(*)` OID 2803; `count(anyelement)` OID 2147). Casts omitted by default.
- **T2 feasibility = Proceed** for sequencing; decision remains `Proposed`
  until product gates. Do not treat Proceed as release approval.
- **PostgreSQL effect-identity majors:** 14–17 (probed); CI primary 17; do not
  claim 12/13/15 without re-probe; 18 out until parser + re-probe.
- **No syntax-name allowlist as trust root.** Promotion requires unique
  catalog-resolved identity **and** manifest membership.
- **No generic volatility trust.** Never implement
  `pg_catalog + provolatile in (i,s) ⇒ Trusted`. Volatility is a **fact**.
- Identity resolver returns **facts only**; trust policy lives in
  application/domain. Callers/resolvers must not claim `Trusted`.
- Fail closed: missing resolver, unknown, multi-match, error, coercion gap,
  non-manifest identity (including `pg_catalog` stable/immutable not in
  manifest), function-backed cast, omitted binary casts → `indeterminate`.
- Rejected ledger always: `current_setting`, `pg_get_*`, UDF effects,
  non-manifest catalog effects, function-backed casts.
- Do not regress MySQL admissible corpus cases; do not change MySQL/TiDB
  behavior for parity.
- No `severity` field; no raw SQL / credentials / connection strings / catalog
  query text in public results.
- MCP must remain without query-access tool.
- No grant evaluation / RLS / masking / SQL rewrite.
- One focused commit per task; no amend/rebase/merge unless instructed.
- Run GitNexus `impact` before editing symbols; `detect_changes` before commit.
- If a kill criterion triggers or implementation needs to widen beyond the T2
  ledger, stop and re-open research; do not invent a weaker trust model.

---

## Task 1b / Task 2 — done

Trust-policy docs and manifest research are complete. **Proceed** with the
closed ledger. Do not re-litigate volatility-class trust. Next executable
production-adjacent work starts at Task 3 (characterization tests).

---

## Task 3 — Characterize PG admission freeze — done

Characterization tests + query-access corpus + T3 Evidence docs are complete.
Commit: `test: characterize query access effect identity candidates`.
Decision remains `Proposed`. No production promotion. Next: Task 4 reason codes.

---

## Task 4 — Unproven effect reason codes

```text
On branch query-access-pure-read-admissibility.

Implement Task 4 from the implementation plan: additive domain reason codes
unproven_operator_effect, unproven_function_effect, unproven_cast_effect,
identity_lookup_failed (optional effect_not_in_trust_manifest) with tests and
ValidateResult compatibility.

Obey T2 ledger. Do not change admission logic yet. No severity field.
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
Closed T2 ledger remains the only future Trusted set.

Gates: go test -tags postgresql on parser + application queryaccess packages.
Commit: feat: extract postgresql query access effect candidates
No push.
```

---

## Task 6 — Resolver contract (facts only) — DONE

```text
DONE on branch query-access-pure-read-admissibility.

T6 delivered an internal, typed, fail-closed EffectIdentityResolver contract:
- domain IdentityStatus (resolved + fail-closed statuses)
- application batch request/result + EffectIdentityFacts (no Trusted)
- ColumnSchema.TypeOID optional (not populated via catalog in T6)
- helpers: ordinal validate/sort/complete, error→lookup_failed, fail-closed reasons
- Analyze NOT wired; public SDK/CLI/HTTP schema unchanged

T6 P1 / P1b amendment (required before T7):
- EffectIdentityResolutionContext MUST be session-complete for promotion:
  Bound + SessionBinding + PathEpoch + DatabaseOID + RoleOID + ServerVersionNum
  (all non-zero); NamespaceSearchOIDs required for unqualified
- explicit schema skips search_path only — not db/role/server/session
- live session mismatch strips ALL (incl. explicit); path-only strips unqualified
- StampFactsFromResolution pins DatabaseOID/ServerVersionNum on facts
- tests: zero fields, role/db/version mismatch, shadowing, TOCTOU, no public leak

T6 is FACTS-ONLY + execution-context-bound. T7 implements the PostgreSQL
catalog adapter under this context. T8 may discuss manifest proof + admission
promotion + optional public injection.

Commit: feat: define query access identity resolver contract
P1 fix: fix: bind effect identity resolution to execution context
```

---

## Task 7 — pg_catalog identity implementation (facts only) — DONE

```text
DONE on branch query-access-pure-read-admissibility.

T7 delivered session-pinned EffectIdentityAdapter (PinnedSession + pg_catalog
exact lookups + live→lookup→live gate). Facts only; not wired into Analyze;
no Trusted / admission.

T8 must implement version-scoped audited manifest proof and controlled
promotion. Do not treat T7 resolved facts as trusted without T8 policy.

Commit: feat: resolve query access effect identity facts
```

## Task 8 — Trust policy + proof engine (next)

```text
On branch query-access-pure-read-admissibility.

Precondition: T7 facts-only session-pinned adapter + T2 closed ledger.
Implement TrustPolicy keyed to the T2 version-scoped audited manifest; wire
proof into classification/admission only when all effects are manifest-trusted
under session-pinned facts. Remove the PostgreSQL hard-stop only under proof.
Do not promote on volatility or bare pg_catalog membership alone.
FORBIDDEN: Trusted from resolver/caller; name allowlists; generic i|s class trust.

Commit: feat: prove trusted query access effects via manifest
No push.
```

---

## Task 8 — Trust policy + proof engine + admission recompute

```text
On branch query-access-pure-read-admissibility.

Implement Task 8 from the plan and design decision order:
- MUST encode T2 Appendix A ledger as the only Trusted set (closed 54 comparison
  ops + count(*)/count(anyelement) + structural BoolExpr; majors 14–17;
  casts omitted by default)
- TrustPolicy.IsTrusted(identity, manifest, version) is the sole Trusted path
- Wire effect candidates + identity facts into Service.Analyze
- Remove unconditional PostgreSQL ban in reclassifyAfterResolution
- Fix sticky indeterminate admission when classification becomes read_only after
  manifest-proven effects
- Relation-only resolver must NOT admit effect-bearing PG SQL without effect proof
- Resolved pg_catalog + stable/immutable NOT in manifest → indeterminate
- current_setting / pg_get_* / UDF stable / function-backed cast → indeterminate
- MySQL path without effect resolver must remain behavior-compatible
- FORBIDDEN: name allowlist trust; universal volatility i|s trust; silent
  expansion beyond T2 ledger

Gates: application/queryaccess tests (with and without postgresql tag),
pkg/deltascope QueryAccess, CLI/HTTP QueryAccess, MCP TestNewServerExposesCoreTools.

Commit: feat: prove trusted effects for query access admission
No push.
```

---

## Task 9 — Corpus expansion

```text
On branch query-access-pure-read-admissibility.

Implement Task 9: expand testdata/query-access per design §8 and Appendix A:
- safe positives only with proof fixtures AND T2 manifest hits (closed set)
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
Document T2 major range 14–17 (CI 17) and closed ledger boundaries.
Fill decision verification evidence; set status Accepted only if gates green.
No severity; no MCP query-access claims; no version bump/release.

Commit: docs: document proven pure-read query access admissibility
No push.
```

---

## Task 11 — Final review (human or agent)

```text
On branch query-access-pure-read-admissibility.

Run Task 11 checklist from the implementation plan. Confirm:
- T2 Proceed ledger is the only Trusted set (no name/volatility-class collapse)
- Majors claimed match probed range 14–17 (or narrower)
- Rejected ledger still fail-closed
- Decision status, corpus counts, sample admissible PG SQL (if any),
  adversarial cases including non-manifest catalog stable, MCP tools still four
- Whether ready for a future release task (do not release)
```
