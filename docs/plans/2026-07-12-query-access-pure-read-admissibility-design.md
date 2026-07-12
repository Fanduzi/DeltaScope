# Design: Query Access Pure-Read Admissibility (Proven Identity)

Date: 2026-07-12
Branch: `query-access-pure-read-admissibility`
Base: `main` @ `4d839b6`
Status: Design + T2 research + T3 characterization complete — no production promotion code
Decision: `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (Proposed)
T2 feasibility: **Proceed** (see Appendix A)
T3: characterization tests lock candidate/rejected ledgers as still unpromoted

## 1. Goal

In fail-closed posture, enable **some** common read-only SELECT shapes to obtain
`read_classification: read_only` and `admission: admissible` when every
effect-bearing construct is proven to resolve to a **catalog identity listed in
an application-maintained, version-scoped, per-item audited trusted-effect
manifest** (primarily PostgreSQL).

Not a goal: make arbitrary SELECT admissible.
Not a goal: trust all `pg_catalog` STABLE/IMMUTABLE builtins by volatility class.

## 2. Current-state diagnosis

### 2.1 Admission pipeline today

```
extractByDialect
  TiDB/MySQL: computeAdmission(classification)  // read_only → Admissible seed
  PostgreSQL: Admission := IndeterminateAdmission always
→ optional SchemaResolver (relations/columns only)
→ append ReasonFunctionEffect if reason "function_call"
→ recomputeAdmission(classification, admission, unresolved, hasResolver)
→ reclassifyAfterResolution(classification, reasonCodes, unresolved, hasResolver, dialect)
→ recomputeAdmission again
→ buildRequirements(mode, ...)
```

`recomputeAdmission` (application):

1. Keep `rejected` if already rejected.
2. If **no resolver** and current admission is `indeterminate` → **keep** it
   (this freezes PostgreSQL's seeded indeterminate when callers omit resolver).
3. If any `unresolved` → `indeterminate`.
4. `not_read_only` → `rejected`.
5. `read_only` → `admissible`.
6. Else → `indeterminate`.

`reclassifyAfterResolution` (application):

1. If classification is **not** already `indeterminate` → return as-is
   (so MySQL `read_only` is never demoted here).
2. If no resolver → stay `indeterminate`.
3. **If dialect == `postgresql` → always stay `indeterminate`** (hard stop).
4. Else (MySQL/TiDB only): if any reason codes remain → stay indeterminate;
   if wildcard/schema unresolved remain → stay indeterminate; else lift to
   `read_only`.

### 2.2 Why PostgreSQL corpus is 22/22 non-admissible

| Layer | Behavior |
|-------|----------|
| Parser classify | Any `A_Expr` or `TypeCast` → indeterminate; any `FuncCall` → indeterminate |
| Extract adapter | Seeds `Admission = indeterminate` always |
| reclassify | Never lifts PG |
| Corpus runner | No `SchemaResolver` → even a future lift would not recompute admission from seeded indeterminate without resolver (step 2 of recompute) |

Additionally: even `simple_select` without operators is seeded with
indeterminate admission; with no resolver, recompute keeps it. MySQL seeds
admission from classification, so operator-free / operator-allowed SELECT can
be admissible without a resolver.

### 2.3 What the parser actually provides

PostgreSQL via `pg_query_go` **parse** AST (`QueryAccessExtractor`):

| Node | Available today | Missing for identity |
|------|-----------------|----------------------|
| `A_Expr` | Presence only (`pgContainsOperatorExpr` returns true immediately). Struct has `Kind`, `Name []*Node`, `Lexpr`, `Rexpr` | No `opno`; name tokens not currently collected; no left/right type OIDs |
| `FuncCall` | Presence only. Struct has `Funcname`, `Args`, `AggStar`, … | No function OID; name path not collected into facts; no arg types; no volatility |
| `TypeCast` | Treated as operator-like presence (`TypeCast` → true in operator walk) | No cast OID; source/target types not bound |
| `BoolExpr` | Recurses into args for operator presence; boolop itself is structural | AND/OR/NOT are not `A_Expr`; currently do not alone force indeterminate unless children do |
| Relation/column | Names, usages, outputs, CTE lineage | Column type OIDs not extracted |

MySQL/TiDB path: comparison operators do **not** force indeterminate;
`FuncCall` / aggregates / window → indeterminate under empty function
allowlist (reason codes may include function markers).

**Conclusion:** parse-time facts can be extended to emit **structural effect
candidates** (names + arg shapes). They **cannot** alone prove runtime
identity. Catalog resolution is mandatory for identity facts. An audited
manifest is mandatory for trust.

### 2.4 What SchemaResolver provides today

`SchemaResolver.ResolveRelation(ctx, dialect, schema, name) → RelationSchema`

- MySQL: `information_schema.tables` / `columns` (name + ordinal only).
- PostgreSQL: `pg_class.relkind` + `pg_attribute.attname/attnum` (no `atttypid`).

No operator/function/cast APIs. Extending relation metadata with type OIDs is
necessary but **not sufficient**; a separate effect-identity API is required.

### 2.5 P1 gap: volatility is not pure-read proof

Prior draft language that trusted `pg_catalog` + `provolatile in ('i','s')`
as a general predicate is **incorrect** for Query Access:

| Claim | Reality |
|-------|---------|
| STABLE ⇒ no table reads | False. STABLE only promises no database **modification**. It may still read tables, GUCs, roles, etc. |
| IMMUTABLE ⇒ no table reads | Not enforced. Catalog label is a claim; the engine does not force "no relation reads." |
| Catalog builtin ⇒ safe for admission | False. Many catalog builtins read hidden context (`current_setting`, `pg_get_*`, …). |
| Requirements still complete | False if an effect can read permission-bearing objects not present in extracted requirements. |

Strict requirements promise coverage of all permission-bearing table/column
reads. Therefore **only** effects whose unique data dependency is the already-
extracted AST operands, with no hidden sources, may enter the trust set — and
only via a **per-item** audit, not a volatility class.

## 3. Threat model (why names and volatility are not enough)

PostgreSQL allows:

- User-defined operators with the same spelling as builtins.
- Operator resolution that depends on operand types and casts.
- `search_path` (and `pg_catalog` insertion rules) affecting which function a bare
  name resolves to.
- User-defined casts that invoke arbitrary functions.
- Aggregates and functions with `SECURITY DEFINER` and arbitrary SQL.
- Catalog functions labeled STABLE/IMMUTABLE that still read non-AST sources.

Therefore:

```
identity_facts := resolve unique OID + namespace + arg types + volatility
                  (from locked catalog context; fail closed on multi-match/error)

trusted := identity_facts complete
        ∧ identity ∈ application_trusted_effect_manifest
        ∧ manifest_entry.version_range covers target PG
        ∧ manifest_entry.audit_proves:
             unique data deps = extracted AST operands only
             ∧ no relation/config/role/file/network hidden reads
             ∧ admitting does not drop permission-bearing requirements
```

Anything less keeps `indeterminate`.

**Forbidden shortcuts:**

```
// NEVER
trusted := namespace == pg_catalog && volatility in (i, s)
trusted := name in ("=", "count", ...)
trusted := caller.Trusted  // resolver/caller claim
```

## 4. Architecture

### 4.1 Separation of concerns

| Layer | Responsibility |
|-------|----------------|
| Parser / extractor | Structural **EffectCandidates** only (names, arg shapes). Never trust. |
| EffectIdentityResolver | Catalog **facts**: OIDs, namespace, types, volatility, cast method. Never sets Trusted. |
| Application/domain trust policy | Intersects resolved identity with the **audited manifest**. Sole authority for Trusted. |
| Admission recompute | Promote only when all effects Trusted **and** existing unresolved/wildcard rules pass. |

Resolver and callers **must not** claim `Trusted`. A fake resolver used in tests
returns facts; tests apply the same policy path.

### 4.2 New application contract (conceptual)

Keep `SchemaResolver` for relations. Add an orthogonal capability:

```text
EffectIdentityResolver (name TBD in implementation)
  ResolveOperator(ctx, req OperatorResolveRequest) (OperatorIdentity, error)
  ResolveFunction(ctx, req FunctionResolveRequest) (FunctionIdentity, error)
  ResolveCast(ctx, req CastResolveRequest) (CastIdentity, error)
```

`QueryAccessRequest` gains optional `EffectIdentityResolver` (or a combined
`CatalogResolver` embedding both). CLI/HTTP remain thin: they do not open DBs
in V1 of this design unless a later task explicitly wires metadata openers;
SDK callers supply resolvers.

Identity result (minimum fields — **facts only**):

```text
OperatorIdentity:
  OperatorOID, Namespace, OperatorName
  LeftTypeOID, RightTypeOID
  ImplementationFunctionOID, ImplementationNamespace
  Volatility  // i | s | v  (fact, not trust)

FunctionIdentity:
  FunctionOID, Namespace, FunctionName, ArgTypeOIDs[]
  Kind  // normal | aggregate | window | ...
  Volatility  // fact, not trust

CastIdentity:
  SourceTypeOID, TargetTypeOID
  CastFunctionOID?   // null if binary / no function
  CastMethod         // f | b | i  (function / binary / inout)
  ImplementationNamespace?
```

**No `Trusted bool` on identity structs.** Trust is computed only by:

```text
TrustPolicy.IsTrusted(identity, manifest, versionContext) → bool
```

Errors / unknown:

- `ErrIdentityUnknown` → map to indeterminate reason, not transport 500 for
  analysis (analysis succeeds with indeterminate).
- Context cancel → fail request as today.
- Do not place DSN, passwords, or SQL in error strings returned to public JSON.

### 4.3 Trusted-effect identity manifest (Phase 1)

#### 4.3.1 Manifest requirements

Phase 1 allows only an **application-maintained** manifest:

- Finite list of effect identities (not open classes).
- Explicit **PostgreSQL version range** per entry or for the whole set.
- Each entry has a **semantic audit note** proving §3 trust predicates.
- Stored in application/domain code or versioned data owned by the product —
  not supplied as an untrusted caller allowlist of names.

#### 4.3.2 Per-entry audit obligations

For each allowed identity, document:

1. Canonical identity key (OID and/or unique name+argtypes under `pg_catalog`
   for the version range).
2. Why the only data dependencies are extracted AST parameters.
3. Why it does not read relations, GUCs, roles, files, network, or other
   hidden sources relevant to permission-bearing data.
4. Why requirements cannot miss a physical source column/table because of this
   effect.
5. Supported PostgreSQL major versions and upgrade/review process.

#### 4.3.3 Phase 1 candidates (T2 ledger — still not automatic trust)

T2 **Proceed** with this closed set. Entries become Trusted only after future
resolve + manifest membership + tests (Tasks 6–9). Full ledger: Appendix A.

- **Structural `BoolExpr` (AND/OR/NOT):** AST control structure, no catalog
  identity. Allowed when every child effect is trusted or absent.
- **Comparison operators (closed):** for each of
  `{=,<>,<,>,<=,>=}` on same-type pairs of
  `{bool,int2,int4,int8,float4,float8,numeric,text,oid}` → **54** operator
  identities (PG17 probe). Each entry keys operator OID + `oprcode` (+ types).
  All probed impls were `provolatile=i` (fact only).
- **Aggregates:** `count(*)` OID **2803**; `count(anyelement)` OID **2147**
  (arg type OID **2276**). Unique aggregate identity required; name `count` is
  not proof.

**Recommended phase-1 default for casts: omit.** Binary casts among core types
are few; cast row OIDs fall in auto-assigned ranges; most useful numeric casts
are function-backed → indeterminate. See rejected ledger.

If research cannot prove a candidate at implement time → leave `indeterminate`
and shrink the positive matrix (do not widen trust).

#### 4.3.4 Explicit exclusions (always indeterminate unless later decision)

- Any non-manifest `pg_catalog` function/operator/cast, including stable and
  immutable ones.
- `current_setting`, `set_config`, and session/GUC readers/writers.
- `pg_get_*` metadata helpers and similar catalog pretty-printers.
- File/network/OS readers (e.g. `pg_read_file`, `pg_ls_dir`).
- User-defined stable/immutable functions, operators, casts.
- Function-backed casts.
- Volatile catalog functions (default).
- Cross-type comparisons needing coercion; `varchar` same-type ops not in the
  closed set (typically text coercion).
- Anything requiring open-ended "all comparison ops" or "all immutable
  builtins" class membership.

#### 4.3.5 Manifest schema (application-owned)

```text
TrustedEffectManifest:
  schema_version: string
  postgresql_major_min: int   // phase-1 claim: 14 (probed)
  postgresql_major_max: int   // phase-1 claim: 17 (probed); CI primary 17
  entries: []TrustedEffectEntry

TrustedEffectEntry:
  kind: operator | aggregate | structural_bool
  // Primary key (prefer OID + verify signature at resolve):
  identity:
    operator_oid? | function_oid?
    namespace_oid?               // expect pg_catalog = 11 on stock installs
    name?                       // verification only, never sole trust root
    arg_type_oids: []oid        // operator left/right or proargtypes
    result_type_oid?
    implementation_function_oid? // oprcode / aggregate fn
  expected_facts:               // fail closed if catalog disagrees
    provolatile?                // recorded fact; not a trust rule
    prokind?
    castmethod?                 // if cast entries ever added: require 'b'
    castfunc_zero?              // if cast: require no function
  audit:
    unique_data_deps: "ast_operands" | "query_row_sources" | ...
    no_hidden_reads: true
    requirements_complete: true
    notes: string
    probed_majors: []int        // e.g. [14,16,17]
```

Resolver returns facts; `TrustPolicy.IsTrusted(facts, manifest, server_major)`
returns true only on unique match + entry audit flags + major in range.

### 4.4 Extraction changes (parser → facts)

PostgreSQL extractor stops treating effects as pure booleans. It emits
**EffectCandidates**:

```text
EffectCandidate:
  Kind: operator | function | cast | bool_struct (optional)
  NamePath: []string          // from A_Expr.Name / FuncCall.Funcname / TypeName
  ArgRoles: left/right/args   // structural
  ArgTypeHints: []TypeHint    // from column type map + const kinds when known
  Provenance: ast path id     // for tests only; never public
```

Rules:

- Collect all candidates; do not drop column lineage when effects exist.
- If a node shape is unsupported for candidate extraction → whole statement
  classification stays indeterminate with `unsupported_node` / new reason.
- **Never** set `read_only` at parse time solely because names match a list.
- **Never** treat volatility labels from the parser (there are none at parse
  time) as trust.

MySQL/TiDB: leave operator policy as-is for phase 1. Function path remains
empty-allowlist indeterminate unless a later phase adds MySQL-safe proof.
**Do not change MySQL/TiDB current behavior.**

### 4.5 Decision order (pseudocode)

```text
function Analyze(req):
  result = extract(req)   // classification may be indeterminate due to effects
  if req.SchemaResolver != nil:
    result = resolveRelations(result)  // wildcards, columns; attach type OIDs when available

  if hasWriteOrLockOrUnsafe(result):
    result.classification = not_read_only
    result.admission = rejected
    return finalize(result)

  candidates = result.EffectCandidates  // empty if none

  if len(candidates) == 0:
    // no effect candidates; use existing non-effect rules (wildcard, etc.)
    result = applyNonEffectIndeterminacy(result)
    result.admission = recomputeAdmission(...)
    return finalize(result)

  if req.EffectIdentityResolver == nil:
    // fail-closed: unproven effects
    result.classification = indeterminate
    result.reason_codes += unproven_* for each kind present
    result.admission = indeterminate
    return finalize(result)

  for c in candidates:
    id, err = resolveIdentity(req.EffectIdentityResolver, c, typeEnv)
    // resolveIdentity returns FACTS only
    if err != nil or id.Unknown:
      result.classification = indeterminate
      result.reason_codes += appropriate unproven / lookup_failed code
      result.admission = indeterminate
      return finalize(result)   // fail closed

    if not TrustPolicy.IsTrusted(id, manifest, versionContext):
      // includes: pg_catalog + stable/immutable but not in manifest
      result.classification = indeterminate
      result.reason_codes += unproven_* / not_in_trust_manifest
      result.admission = indeterminate
      return finalize(result)

  // all effects proven trusted via manifest
  if still has unresolved permission-bearing refs / wildcards / ambiguity:
    result.classification = indeterminate  // or keep read_only only if policy says
    result.admission = indeterminate
    return finalize(result)

  result.classification = read_only
  result.admission = recomputeAdmission(read_only, ..., hasRelationResolver)
  // Note: for PostgreSQL, recomputeAdmission must be allowed to promote when
  // classification is read_only even if admission was seeded indeterminate,
  // when either resolver is present OR effects were proven (implementation must
  // fix the "no resolver freezes seeded indeterminate" interaction carefully:
  // effect proof implies catalog access; relation resolver may still be required
  // for wildcards).
  return finalize(result)
```

**Critical fix vs today:** remove the unconditional
`if dialect == "postgresql" { return Indeterminate }` lift ban, and replace it
with **manifest-gated** promotion. Do not lift on relation resolver alone.
Do not lift on catalog volatility alone.

**Admission freeze fix:** seeded PostgreSQL `indeterminate` admission must not
remain sticky when classification becomes `read_only` after proof, even if the
historical `!hasResolver && current==indeterminate` guard would freeze it.
Preferred rule:

```text
if classification == read_only and unresolved empty and effectsProvenViaManifest:
  admission = admissible
elif classification == not_read_only:
  admission = rejected
else:
  admission = indeterminate
```

Relation-only resolver without effect proof must **not** produce PG
`admissible` for effect-bearing SQL.

### 4.6 Catalog resolution (PostgreSQL)

#### 4.6.1 Resolution context

Caller must lock an execution-equivalent context:

- database
- `search_path` policy: for pure-read promotion, **prefer resolving only in
  `pg_catalog`** for operators/functions (ignore user schemas for trust), OR
  require fully schema-qualified function/operator names that resolve inside
  `pg_catalog`
- role used for catalog reads (should need only SELECT on `pg_catalog`)

If the platform executes queries under a different path/types, proof is void
(document as caller obligation; same as foundation relation resolution).

#### 4.6.2 Minimal metadata queries (illustrative — facts only)

Extend relation columns:

```sql
select a.attname, a.attnum, a.atttypid
from pg_catalog.pg_attribute a
join pg_catalog.pg_class c on c.oid = a.attrelid
join pg_catalog.pg_namespace n on n.oid = c.relnamespace
where n.nspname = $1 and c.relname = $2
  and a.attnum > 0 and not a.attisdropped
order by a.attnum;
```

Operator (exact type match first):

```sql
select o.oid, n.nspname, o.oprname, o.oprleft, o.oprright, o.oprcode,
       p.pronamespace, pn.nspname as pro_nsp, p.provolatile, p.prokind
from pg_catalog.pg_operator o
join pg_catalog.pg_namespace n on n.oid = o.oprnamespace
join pg_catalog.pg_proc p on p.oid = o.oprcode
join pg_catalog.pg_namespace pn on pn.oid = p.pronamespace
where n.nspname = 'pg_catalog'
  and o.oprname = $1
  and o.oprleft = $2
  and o.oprright = $3;
```

If zero rows → try only **documented** safe binary coercions later; default
phase 1: **indeterminate** (no coercion graph).

Function:

```sql
select p.oid, n.nspname, p.proname, p.proargtypes, p.provolatile, p.prokind
from pg_catalog.pg_proc p
join pg_catalog.pg_namespace n on n.oid = p.pronamespace
where n.nspname = 'pg_catalog'
  and p.proname = $1
  and p.proargtypes = $2::oidvector;  -- exact
```

Cast:

```sql
select c.oid, c.castsource, c.casttarget, c.castfunc, c.castcontext, c.castmethod
from pg_catalog.pg_cast c
where c.castsource = $1 and c.casttarget = $2;
```

Then load `castfunc` through `pg_proc` **as facts** when present.
Trust policy:

```text
// Correct (phase 1)
if castmethod indicates function-backed / castfunc != 0:
  → not trusted (default; keep indeterminate)
if binary cast with no function:
  → trusted only if identity in manifest with audit proof
else:
  → indeterminate

// For operator/function identities:
trusted := unique_resolution
        && identity ∈ trusted_effect_manifest
        && version_in_range
// NOT: pro_nsp == pg_catalog && provolatile in (i,s)
```

Resolver error, unknown OID, multi-match, incomplete type/coercion → fail closed
(`indeterminate`).

### 4.7 Type inference (bounded)

Phase 1 type environment:

- Column refs: type OID from resolved relation metadata.
- Integer/string/bool/float consts: map to a small set of default OIDs
  (`int4`/`int8`, `text`, `bool`, `float8`) carefully; if const kind unknown →
  unproven.
- Column vs column: both types known required.
- No expression algebra beyond direct operands of candidates.
- If operand type missing → that candidate unproven → statement indeterminate.

This intentionally under-approximates (more indeterminate) rather than
over-approximates (false admissible).

## 5. Support matrix (phase 1)

### 5.1 Eligible for promotion (when identity + manifest succeed)

| Shape | Notes |
|-------|--------|
| `SELECT cols FROM t` | Already structural read_only; admission must not stick indeterminate once classification is read_only and no unresolved (with or without effects) |
| `SELECT cols FROM t WHERE col <op> const` | `<op>` unique catalog identity **and** in comparison manifest |
| `WHERE` with `AND`/`OR`/`NOT` combining proven comparisons | BoolExpr structural (documented) |
| `INNER/LEFT/RIGHT JOIN ... ON` proven comparisons | Same operator proof + manifest |
| Simple `USING` / `NATURAL` join without extra unproven exprs | As today for structure; still need relation metadata if wildcards |
| `COUNT(*)` / `COUNT(col)` | Only if aggregate identity is in manifest with full audit; else indeterminate |

### 5.2 Remain indeterminate (explicit)

| Shape | Reason |
|-------|--------|
| Any operator/function/cast without resolver | unproven |
| Resolved `pg_catalog` + stable/immutable **not** in manifest | not trusted |
| User-defined operator/function/cast | not trusted |
| Name collision / multi-match / needs coercion | unproven |
| `current_setting` / `pg_get_*` / hidden context readers | not trusted |
| Function-backed cast | not trusted (phase 1 default) |
| Volatile catalog functions | unproven / not trusted |
| `SELECT *` without full expansion | schema_unavailable / unresolved |
| Ambiguous unqualified columns | ambiguous_reference |
| Resolver errors | identity_lookup_failed |
| Subqueries with unproven effects | recursive fail-closed |
| Window functions, SRFs, `generate_series` until proven | unproven |
| Data-modifying CTE, locking, `INTO`, DDL/DML | not_read_only / rejected |
| View expansion to base tables | deferred |
| Dynamic SQL | deferred |

### 5.3 MySQL/TiDB

- Keep existing admissible SELECT+WHERE without function calls.
- Do not add name-only function allowlists "for parity".
- Do not change current MySQL/TiDB behavior in this milestone.
- Optional later: identity-backed function promotion with a MySQL-specific
  proof model (not phase 1).

## 6. Public surfaces

| Surface | Behavior |
|---------|----------|
| SDK | Accept optional effect-identity resolver; same `AnalyzeQueryAccess` |
| HTTP | Optional later wiring only if request model can reference server-side catalog safely; phase 1 may remain relation/default_schema only + document that proof requires SDK resolver |
| CLI | Phase 1: continue diagnostic JSON without DB open unless a later task adds explicit metadata flags; document limitation |
| MCP | **No** query-access tool |

Cross-surface rule: transports only serialize `domain.Result`; no divergent
admission logic.

## 7. Privacy / no-leak

Public JSON may include:

- relation/column names (existing)
- bounded reason codes
- mode, dialect, classification, admission

Must not include:

- raw/normalized SQL
- literals beyond what identifiers already imply (prefer no literal echo)
- connection strings, user/password, host
- catalog query text
- OID dumps unless product later decides OIDs are safe machine fields
  (default: OIDs stay internal to resolver/trust engine)

No `severity` field.

## 8. Corpus / adversarial matrix (required)

### 8.1 Safety positives (expect admissible when proof fixtures + manifest hit)

1. PG `SELECT id, name FROM app.users`
2. PG `SELECT id FROM app.users WHERE id = 1` with int equality identity in
   manifest
3. PG `WHERE active AND id > 0` (bool + comparisons in manifest)
4. PG simple `JOIN ... ON a.id = b.user_id` with types proven and ops in
   manifest
5. PG `SELECT count(*) FROM app.users` **only if** `count` aggregate identity
   is in manifest; otherwise expect indeterminate (positive is conditional)
6. MySQL regression: existing admissible fixtures still pass

### 8.2 Adversarial / fail-closed (expect indeterminate or rejected)

1. No resolver + `WHERE id = 1` → indeterminate
2. Resolver error (any) → indeterminate; no secret leak
3. Unknown OID / no catalog row → indeterminate
4. Multi-match / ambiguous resolution → indeterminate
5. Incomplete type / coercion gap → indeterminate
6. **`pg_catalog` + stable (or immutable) identity not in manifest** →
   indeterminate
7. **`current_setting(...)`** (or equivalent hidden GUC/context effect) →
   indeterminate
8. **`pg_get_*` metadata helpers** → indeterminate
9. **User-defined stable function / operator / cast** → indeterminate
10. **Function-backed cast** → indeterminate
11. Custom operator `=` in non-catalog schema preferred by path → indeterminate
12. Custom function named `count` in user schema → indeterminate
13. Ambiguous column → indeterminate
14. Wildcard without metadata → indeterminate
15. Unknown function `evil()` → indeterminate
16. `SELECT ... FOR UPDATE` → rejected / not_read_only
17. Data-modifying CTE → rejected / not_read_only
18. Schema shadowing fixture: user `pg_catalog`-looking name that is not
    true catalog OID trust → indeterminate
19. **Only a single manifest-listed effect may promote** under full
    relation/column metadata; mixing with any non-manifest effect → whole
    statement indeterminate
20. **Strict requirements must not omit any known physical source column**
    when admission is admissible (completeness check fixtures)

### 8.3 No-leak fixtures

- Malformed SQL errors contain no SQL text
- Resolver error messages scrubbed in public mapping tests
- Results/errors contain no credentials, connection strings, catalog query text
- No `severity` field anywhere on the public query-access result

## 9. Alternatives considered

| Alternative | Why rejected |
|-------------|--------------|
| Syntax allowlist for `=`, `count` | Unsafe under overload/shadowing |
| Trust all `pg_catalog` + volatility `i\|s` | STABLE/IMMUTABLE ≠ no hidden reads; breaks requirements completeness |
| Lift PG whenever SchemaResolver present | Relation metadata ≠ effect purity |
| Always admissible for SELECT | Violates fail-closed foundation |
| Caller-supplied Trusted bit | Callers must not self-assert trust |
| MCP tool for access | No agent workflow; non-goal |
| Full planner/type coercion | Out of scope; kill if required for phase 1 matrix |
| Open-ended "all comparison ops" class | Not a per-item audited manifest |

## 10. Kill criteria (audit spike only)

Stop product implementation and leave research notes if:

1. Bounded type inference cannot cover the phase-1 matrix without full
   planner semantics.
2. Unique `pg_catalog` operator/function match cannot be obtained without
   accepting user-schema shadowing.
3. Catalog access forces privilege or leak patterns incompatible with no-leak
   contracts.
4. Any proposed design collapses to **name allowlists** without OID binding.
5. Any proposed design collapses to a **generic volatility allowlist**
   (`pg_catalog` + `i|s`) without per-item audited, version-scoped identities.
6. A **bounded, version-scoped, maintainable** trusted-effect manifest cannot
   be produced for the phase-1 candidates (maintenance forces open-ended class
   trust, or semantic audit cannot prove no hidden reads / requirements
   completeness).

## 11. Worth implementing?

**Yes**, contingent on:

1. Phase-1 matrix staying inside exact-type catalog matches,
2. Fail-closed unknowns and non-manifest identities,
3. A maintainable, version-scoped, per-item audited trusted-effect manifest.

The product already declared the query platform scenario; PostgreSQL is
unusable for admission without this work. Identity facts + manifest trust
satisfies the safety constraint that name allowlists and volatility-class
allowlists both violate.

If kill criteria fire, keep an audit spike only — do not ship a weaker trust
root.

## Appendix A — T2 research: manifest feasibility (2026-07-12)

### A.1 Decision

**Proceed.** A bounded, version-scoped, per-item audited effect identity
manifest is feasible for phase 1 without name or volatility-class allowlists.

Not implemented in T2. Decision record remains `Proposed` until product gates
land.

### A.2 Repo version matrix

| Surface | PostgreSQL major evidence |
|---------|---------------------------|
| Docker E2E compose | **17** only (`postgres:17`) |
| Metadata-aware audit product claim | **12+** (CHANGELOG); not multi-version CI for query-access effects |
| Parser (`pg_query_go` v6 / libpg_query) | **17** grammar; **18** parser support deferred |
| T2 ephemeral probes | **14.23**, **16.14**, **17.10** (Docker official images) |
| T2 **phase-1 effect-identity claim** | **14–17** (probed). CI primary remains **17**. |
| **Not claimed without further probe** | 12, 13, 15 |
| Out of phase-1 parse/promotion | 18+ until parser + re-probe |

### A.3 Identity fields (runtime proof)

| Catalog | Fields for proof |
|---------|------------------|
| `pg_operator` | `oid`, `oprnamespace`, `oprname`, `oprleft`, `oprright`, `oprresult`, `oprcode` |
| `pg_proc` | `oid`, `pronamespace`, `proname`, `proargtypes`, `prorettype`, `provolatile`, `prokind` |
| `pg_aggregate` | `aggfnoid`, `aggkind`, `aggtransfn`, `aggfinalfn` (audit context) |
| `pg_cast` | `castsource`, `casttarget`, `castfunc`, `castmethod`, `castcontext` (not cast row OID as sole key) |
| `pg_type` / `pg_namespace` | type OID + namespace OID facts |

### A.4 OID stability

- Official: released manually assigned OIDs are not renumbered; auto-assigned
  bootstrap OIDs (~10000–16383) are installation-unstable.
- Probe: candidate type/operator/impl/aggregate OIDs matched across 14/16/17
  for the closed set (0 mismatches on identity keys).
- Binary `pg_cast.oid` values observed ≥10000 → key by type pair + method.

### A.5 Candidate ledger (minimal promote set)

**Structural (no OID):**

- `BoolExpr` AND / OR / NOT — pure AST; children must be trusted or absent.

**Operators (54):** same-type pairs for types
`bool(16), int2(21), int4(23), int8(20), float4(700), float8(701),
numeric(1700), text(25), oid(26)` and names
`=,<>,<,>,<=,>=`.

Illustrative stable samples (14=16=17):

- `=`(bool,bool) op **91** / `booleq` **60**
- `=`(int4,int4) op **96** / `int4eq` **65**
- `=`(int8,int8) op **410** / `int8eq` **467**
- `=`(text,text) op **98** / `texteq` **67**
- `=`(numeric,numeric) op **1752** / `numeric_eq` **1718**

All 54 probed implementations: `prokind=f`, `provolatile=i`, namespace
`pg_catalog`. Volatility recorded as fact only.

**Semantic audit (operators):** unique data dependency is left/right AST
operands; pure comparison C builtins; no table/GUC/role/file/network reads;
cannot omit permission-bearing columns already required by operand extraction.

**Aggregates:**

- `count(*)` function OID **2803**, empty `proargtypes`, `prokind=a`,
  `provolatile=i` (fact)
- `count(anyelement)` function OID **2147**, arg type **2276**, same kind/vol

**Semantic audit (count):** aggregates only over already-extracted query row
sources / argument expression; does not open additional relations; name
`count` alone is never sufficient (user-defined overload risk).

### A.6 Rejected ledger

| Class | Disposition |
|-------|-------------|
| Non-manifest `pg_catalog` op/fn/cast (incl. stable/immutable) | indeterminate |
| `current_setting` / `set_config` | indeterminate (hidden GUC/session) |
| `pg_get_*` helpers | indeterminate (catalog metadata) |
| `pg_read_file` / `pg_ls_dir` / similar | indeterminate |
| User-defined stable/immutable/volatile effects | indeterminate |
| Function-backed casts (`castmethod=f`) | indeterminate |
| Binary casts (phase-1 default omit) | indeterminate until type-pair entries + tests |
| Coercion-required / multi-match / unknown OID | indeterminate |
| Syntax/name/schema allowlist | forbidden |
| `pg_catalog + i\|s` class allowlist | forbidden |

### A.7 Privileges, context, no-leak

- Catalog SELECT on `pg_catalog` (same privilege class as existing relation
  resolver).
- Lock search_path/database/role to execution; prefer `pg_catalog`-only resolve
  for trust.
- No DSN, credentials, catalog query text, raw SQL, or `severity` in public
  results. Probes and design may store abstract OIDs and counts only.

### A.8 Characterization scope (T3 evidence, not T2 code)

**T3 done (tests + corpus only — not product support):**

- Freeze today’s PG effect hard-stop: candidate comparisons, `COUNT(*)` /
  `COUNT(col)`, casts, rejected ledger, no/error/incomplete resolver → remain
  `indeterminate` classification and admission.
- Structural BoolExpr AND/OR/NOT documented as non-catalog identity (column-only
  forms may classify `read_only`; wrapping candidates stays indeterminate).
- MySQL/TiDB operator-bearing SELECT regression locked admissible.
- No-leak / no-severity assertions on characterization paths.
- Decision remains `Proposed`. Characterization does **not** authorize Trusted
  promotion. PostgreSQL promotion stays forbidden until identity resolver +
  audited manifest + proof engine complete.

**Later (not T3):**

- Positives only for closed set under fake/real identity + manifest.
- Adversarial under real identity facts: non-manifest catalog stable still
  indeterminate; requirements completeness under promotion.

### A.9 Worth implementing after T2?

Yes, contingent on implementing **only** the closed ledger, major range
**14–17** (CI **17**), fail-closed elsewhere, and never inventing name or
volatility-class trust roots. Expanding majors or entries requires re-probe +
re-audit, not silent widening.
