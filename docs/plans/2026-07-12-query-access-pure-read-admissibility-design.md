# Design: Query Access Pure-Read Admissibility (Proven Identity)

Date: 2026-07-12
Branch: `query-access-pure-read-admissibility`
Base: `main` @ `4d839b6`
Status: Design only (Task 1) — no production code
Decision: `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md` (Proposed)

## 1. Goal

In fail-closed posture, enable **some** common read-only SELECT shapes to obtain
`read_classification: read_only` and `admission: admissible` when every
effect-bearing construct is proven to resolve to a **trusted builtin
implementation identity** (primarily PostgreSQL).

Not a goal: make arbitrary SELECT admissible.

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
identity. Catalog resolution is mandatory for promotion.

### 2.4 What SchemaResolver provides today

`SchemaResolver.ResolveRelation(ctx, dialect, schema, name) → RelationSchema`

- MySQL: `information_schema.tables` / `columns` (name + ordinal only).
- PostgreSQL: `pg_class.relkind` + `pg_attribute.attname/attnum` (no `atttypid`).

No operator/function/cast APIs. Extending relation metadata with type OIDs is
necessary but **not sufficient**; a separate effect-identity API is required.

## 3. Threat model (why names are not enough)

PostgreSQL allows:

- User-defined operators with the same spelling as builtins.
- Operator resolution that depends on operand types and casts.
- `search_path` (and `pg_catalog` insertion rules) affecting which function a bare
  name resolves to.
- User-defined casts that invoke arbitrary functions.
- Aggregates and functions with `SECURITY DEFINER` and arbitrary SQL.

Therefore:

```
proof := resolved_oid ∈ trusted_pg_catalog_set
         ∧ implementation_oid trusted
         ∧ volatility allowed
         ∧ resolution used locked context + typed operands
```

Anything less keeps `indeterminate`.

## 4. Architecture

### 4.1 New application contract (conceptual)

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

Identity result (minimum fields, public or internal as needed):

```text
OperatorIdentity:
  OperatorOID, Namespace (must be pg_catalog), OperatorName
  LeftTypeOID, RightTypeOID
  ImplementationFunctionOID, ImplementationNamespace
  Volatility  // i | s | v
  Trusted bool  // computed by policy, not caller claim

FunctionIdentity:
  FunctionOID, Namespace, FunctionName, ArgTypeOIDs[]
  Kind  // normal | aggregate | window | ...
  Volatility
  Trusted bool

CastIdentity:
  SourceTypeOID, TargetTypeOID, CastFunctionOID?, Namespace?
  Trusted bool
```

Errors / unknown:

- `ErrIdentityUnknown` → map to indeterminate reason, not transport 500 for
  analysis (analysis succeeds with indeterminate).
- Context cancel → fail request as today.
- Do not place DSN, passwords, or SQL in error strings returned to public JSON.

### 4.2 Extraction changes (parser → facts)

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

MySQL/TiDB: leave operator policy as-is for phase 1. Function path remains
empty-allowlist indeterminate unless a later phase adds MySQL-safe proof.

### 4.3 Decision order (pseudocode)

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
    if err != nil or id.Unknown or not id.Trusted:
      result.classification = indeterminate
      result.reason_codes += appropriate unproven / lookup_failed code
      result.admission = indeterminate
      return finalize(result)   // fail closed on first unproven effect

  // all effects proven trusted
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
with proof-gated promotion. Do not lift on relation resolver alone.

**Admission freeze fix:** seeded PostgreSQL `indeterminate` admission must not
remain sticky when classification becomes `read_only` after proof, even if the
historical `!hasResolver && current==indeterminate` guard would freeze it.
Preferred rule:

```text
if classification == read_only and unresolved empty and effectsProven:
  admission = admissible
elif classification == not_read_only:
  admission = rejected
else:
  admission = indeterminate
```

Relation-only resolver without effect proof must **not** produce PG
`admissible` for effect-bearing SQL.

### 4.4 Catalog resolution (PostgreSQL)

#### 4.4.1 Resolution context

Caller must lock an execution-equivalent context:

- database
- `search_path` policy: for pure-read promotion, **prefer resolving only in
  `pg_catalog`** for operators/functions (ignore user schemas for trust), OR
  require fully schema-qualified function/operator names that resolve inside
  `pg_catalog`
- role used for catalog reads (should need only SELECT on `pg_catalog`)

If the platform executes queries under a different path/types, proof is void
(document as caller obligation; same as foundation relation resolution).

#### 4.4.2 Minimal metadata queries (illustrative)

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

Then resolve `castfunc` through `pg_proc` trust policy when present.

Trust predicate:

```text
trusted := pro_nsp == 'pg_catalog'
        && (provolatile in ('i','s'))   -- phase 1 default
        && not rejected_by_kind(prokind)
```

### 4.5 Type inference (bounded)

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

### 5.1 Eligible for promotion (when proof succeeds)

| Shape | Notes |
|-------|--------|
| `SELECT cols FROM t` | Already structural read_only; admission must not stick indeterminate once classification is read_only and no unresolved (with or without effects) |
| `SELECT cols FROM t WHERE col <op> const` | `<op>` proven trusted comparison in `pg_catalog` |
| `WHERE` with `AND`/`OR`/`NOT` combining proven comparisons | BoolExpr structural |
| `INNER/LEFT/RIGHT JOIN ... ON` proven comparisons | Same operator proof |
| Simple `USING` / `NATURAL` join without extra unproven exprs | As today for structure; still need relation metadata if wildcards |
| `COUNT(*)` / `COUNT(col)` when function OID proven trusted aggregate | Aggregate `prokind` allowed |
| Other frozen builtins only if OID-proven in tests (e.g. `length(text)`) | Explicit corpus, not open-ended |

### 5.2 Remain indeterminate (explicit)

| Shape | Reason |
|-------|--------|
| Any operator/function/cast without resolver | unproven |
| User-defined operator/function/cast | not trusted |
| Name collision / multi-match / needs coercion | unproven |
| Volatile catalog functions (default) | unproven |
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

## 8. Corpus matrix (required)

### 8.1 Safety positives (expect admissible when proof fixtures supplied)

1. PG `SELECT id, name FROM app.users`
2. PG `SELECT id FROM app.users WHERE id = 1` with int equality proven
3. PG `WHERE active AND id > 0` (bool + comparisons)
4. PG simple `JOIN ... ON a.id = b.user_id` with types proven
5. PG `SELECT count(*) FROM app.users` with count proven
6. MySQL regression: existing admissible fixtures still pass

### 8.2 Adversarial / fail-closed (expect indeterminate or rejected)

1. No resolver + `WHERE id = 1` → indeterminate
2. Custom operator `=` in non-catalog schema preferred by path → indeterminate
3. Custom function named `count` in user schema → indeterminate
4. User-defined cast on column type → indeterminate
5. Ambiguous column → indeterminate
6. Wildcard without metadata → indeterminate
7. Resolver error mid-lookup → indeterminate, no secret leak
8. Unknown function `evil()` → indeterminate
9. `SELECT ... FOR UPDATE` → rejected / not_read_only
10. Data-modifying CTE → rejected / not_read_only
11. Schema shadowing fixture: user `pg_catalog`-looking name that is not
    true catalog OID trust → indeterminate
12. Type mismatch requiring coercion not implemented → indeterminate

### 8.3 No-leak fixtures

- Malformed SQL errors contain no SQL text
- Resolver error messages scrubbed in public mapping tests

## 9. Alternatives considered

| Alternative | Why rejected |
|-------------|--------------|
| Syntax allowlist for `=`, `count` | Unsafe under overload/shadowing |
| Lift PG whenever SchemaResolver present | Relation metadata ≠ effect purity |
| Always admissible for SELECT | Violates fail-closed foundation |
| MCP tool for access | No agent workflow; non-goal |
| Full planner/type coercion | Out of scope; kill if required for phase 1 matrix |

## 10. Kill criteria (audit spike only)

Stop product implementation and leave research notes if:

1. Bounded type inference cannot cover the phase-1 matrix without full
   planner semantics.
2. Unique `pg_catalog` operator/function match cannot be obtained without
   accepting user-schema shadowing.
3. Catalog access forces privilege or leak patterns incompatible with no-leak
   contracts.
4. Any proposed design collapses to name allowlists without OID binding.

## 11. Worth implementing?

**Yes**, contingent on phase-1 matrix staying inside exact-type `pg_catalog`
matches and fail-closed unknowns. The product already declared the query
platform scenario; PostgreSQL is unusable for admission without this work.
Identity-proof design satisfies the safety constraint that name allowlists
violate.
