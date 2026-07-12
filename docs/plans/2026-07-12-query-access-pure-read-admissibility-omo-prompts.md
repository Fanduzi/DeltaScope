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
  requires catalog-proven identity (OID-bound `pg_catalog` + allowed volatility).
- Fail closed: missing resolver, unknown, multi-match, error, coercion gap →
  `indeterminate`.
- Do not regress MySQL admissible corpus cases.
- No `severity` field; no raw SQL / credentials in public results.
- MCP must remain without query-access tool.
- One focused commit per task; no amend/rebase/merge unless instructed.
- Run GitNexus `impact` before editing symbols; `detect_changes` before commit.
- If a kill criterion from the design triggers, stop and report; do not invent a
  weaker trust model.

---

## Task 2 — Characterize PG admission freeze

```text
On branch query-access-pure-read-admissibility in /Users/fan/GolangProjects/DeltaScope.

Implement Task 2 from docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md.

Add characterization tests only (prefer no production code) proving:
1) PostgreSQL SELECT with WHERE equality is classification+admission indeterminate without effect identity resolver.
2) MySQL equivalent remains admissible (regression).
3) reclassifyAfterResolution currently does not lift PostgreSQL indeterminate.

Follow design diagnosis in docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md §2.

Gates: go test ./internal/application/queryaccess/ -count=1; go test -tags postgresql ./internal/application/queryaccess/ -count=1; git diff --check; gitnexus detect_changes.

Commit: test: characterize query access pg admission freeze
No co-authors. No push.
```

---

## Task 3 — Unproven effect reason codes

```text
On branch query-access-pure-read-admissibility.

Implement Task 3 from the implementation plan: additive domain reason codes
unproven_operator_effect, unproven_function_effect, unproven_cast_effect,
identity_lookup_failed with tests and ValidateResult compatibility.

Do not change admission logic yet. No severity field.

GitNexus impact on ReasonCode / ValidateResult before edits.
Commit: feat: add query access unproven effect reason codes
No push.
```

---

## Task 4 — Extract PG effect candidates

```text
On branch query-access-pure-read-admissibility.

Implement Task 4: PostgreSQL extractor emits structured effect candidates
(A_Expr / FuncCall / TypeCast name paths + arg structure). Classification must
remain indeterminate for unproven effects. Do NOT add name allowlists. Do NOT
promote admission.

Map candidates through extract_postgresql.go without trusting names.

Gates: go test -tags postgresql on parser + application queryaccess packages.
Commit: feat: extract postgresql query access effect candidates
No push.
```

---

## Task 5 — Resolver contract + type OIDs

```text
On branch query-access-pure-read-admissibility.

Implement Task 5: extend SchemaResolver column metadata with type OIDs for
PostgreSQL; add EffectIdentityResolver application contract and public SDK
optional surface if needed; fake resolvers for tests. Update package README if
exports change.

Do not implement live pg_operator queries yet (Task 6).
Do not change MCP tools.

Commit: feat: add query access effect identity resolver contract
No push.
```

---

## Task 6 — pg_catalog identity implementation

```text
On branch query-access-pure-read-admissibility.

Implement Task 6: PostgreSQL EffectIdentityResolver against pg_catalog
(pg_operator / pg_proc / pg_cast) with exact type match, trust only pg_catalog
+ volatility i|s, scrub secrets from errors. Unknown/multi-match → not trusted.

If unique identity requires a full coercion planner for the phase-1 matrix,
STOP and report kill criterion instead of adding name allowlists.

Commit: feat: resolve postgresql effect identities from pg_catalog
No push.
```

---

## Task 7 — Proof engine + admission recompute

```text
On branch query-access-pure-read-admissibility.

Implement Task 7 from the plan and design §4.3:
- Wire effect candidates + identity proof into Service.Analyze
- Remove unconditional PostgreSQL ban in reclassifyAfterResolution
- Fix sticky indeterminate admission when classification becomes read_only after proof
- Relation-only resolver must NOT admit effect-bearing PG SQL without effect proof
- MySQL path without effect resolver must remain behavior-compatible

Gates: application/queryaccess tests (with and without postgresql tag),
pkg/deltascope QueryAccess, CLI/HTTP QueryAccess, MCP TestNewServerExposesCoreTools.

Commit: feat: prove trusted effects for query access admission
No push.
```

---

## Task 8 — Corpus expansion

```text
On branch query-access-pure-read-admissibility.

Implement Task 8: expand testdata/query-access per design §8 (safe positives with
proof fixtures; adversarial unknowns, custom operator/function/cast, ambiguity,
wildcard, resolver error, no-leak). Keep MySQL regressions green.

make query-access-corpus-gates must pass.

Commit: test: expand query access pure read corpus
No push.
```

---

## Task 9 — Docs + accept decision

```text
On branch query-access-pure-read-admissibility.

Implement Task 9: update query-access reference + recipe (EN/ZH) to replace
"PostgreSQL admission is always indeterminate" with proof-gated wording.
Fill decision verification evidence; set status Accepted only if gates green.
No severity; no MCP query-access claims; no version bump/release.

Commit: docs: document proven pure-read query access admissibility
No push.
```

---

## Task 10 — Final review (human or agent)

```text
On branch query-access-pure-read-admissibility.

Run Task 10 checklist from the implementation plan. Confirm kill criteria did
not force a name-only design. Report: corpus counts, sample admissible PG SQL,
adversarial cases, MCP tool list still four, decision status, whether ready for
a future release task (do not release).
```
