# Decision: Query Access Pure-Read Admissibility via Proven Identity

Date: 2026-07-12
Status: Proposed
Related milestone/version: (unassigned; branch `query-access-pure-read-admissibility`)
Related commits:
- Design + trust-policy docs on branch `query-access-pure-read-admissibility`
- T2 research commit: `docs: research query access effect identity manifest`
- T3 characterization commit: `test: characterize query access effect identity candidates`
- T4 reason codes commit: `feat: explain unproven query access effects`
- T4 follow-up: `fix: complete query access effect reason traversal`
- T5 candidates commit: `feat: extract query access effect candidates`
- T6 resolver contract commit: `feat: define query access identity resolver contract`
- T6 P1 execution-context fix: `fix: bind effect identity resolution to execution context`
- T6 P1b session completeness: `fix: require full identity resolution session binding`
- T7 catalog adapter: `feat: resolve query access effect identity facts`
- T8 manifest proof: `feat: prove bounded query access effects`
Related tests:
- T3: `internal/application/queryaccess/effect_identity_characterization_postgresql_tag_test.go`
- T3: `internal/application/queryaccess/effect_identity_mysql_tidb_regression_test.go`
- T3: `internal/infrastructure/parser/postgresql/query_access_effect_identity_postgresql_tag_test.go`
- T3: extended `testdata/query-access/postgresql/` corpus (structural / candidate / rejected)
- T4: domain reason constant + identity-failure mapping + normalize order tests
- T4: `internal/application/queryaccess/unproven_effect_reasons_postgresql_tag_test.go`
- T4: `internal/application/queryaccess/unproven_effect_mysql_tidb_regression_test.go`
- T4: parser unproven reason emission tests
- T4: SDK/CLI/HTTP postgresql-tag passthrough + no-leak tests
- T4: PG corpus expected fixtures record reason ids only
- T6: `identity_resolver_test.go`, `identity_resolver_no_invoke_test.go`, domain status mapping tests
- T6: PostgreSQL Analyze freeze (no identity_* from Analyze; public JSON no OIDs/facts)
- T6 P1: `identity_resolver_context_test.go` (unbound unqualified, shadowing, overload, TOCTOU, no public context leak)
- T7: `effect_identity_resolver_test.go` (fake pinned catalog); optional
  `effect_identity_resolver_integration_test.go` (PG17 Docker)
- T8: `trust_policy_test.go` (17 tests: all trust decisions, manifest validation, hash determinism, PG17 manifest validity)
- T8: `trusted_service_test.go` (constructor validation, promotion logic, PG hard-stop removal verification)
- Planned (T8): positive corpus under fake/real identity resolver + manifest promotion
- T2: no production/test code; ephemeral catalog probes only under `/tmp` (not committed)
Related docs:
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md` (T2 appendix)
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md`
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-omo-prompts.md`
- Supersedes boundaries only where noted in V1 foundation:
  `docs/decisions/2026-07-11-query-access-analysis-foundation.md`

## Context

v0.380.0 shipped Query Access Analysis as a separate public capability (SDK,
CLI, HTTP; MCP intentionally without a query-access tool). The stated target
user is a query platform that must decide, before execution, whether SQL is
demonstrably read-only and which table/column permissions the caller must
hold.

On PostgreSQL, the foundation is fail-closed in a way that makes the target
scenario largely unusable:

- The parser path treats any `A_Expr` / `TypeCast` and any `FuncCall` as
  classification `indeterminate` (presence check only; no operator OID, no
  function OID, no volatility, no namespace).
- `AnalyzePostgreSQL` always seeds `Admission = indeterminate`.
- `reclassifyAfterResolution` **never lifts PostgreSQL** out of
  `indeterminate`, even when a `SchemaResolver` is present:
  `if dialect == "postgresql" { return Indeterminate }`.
- Corpus: all 22 PostgreSQL fixtures have `admission: indeterminate`
  (0 admissible). MySQL corpus has 10 admissible cases for operator-bearing
  SELECT without function calls.
- Recipe docs currently state that PostgreSQL admission is always
  indeterminate.

The foundation decision already anticipated a future path: resolve operator
and cast identities against the catalog and replace the empty function
allowlist with a proven pure-effect policy. That future path is this
proposal.

**Hard safety constraint:** a syntactic token such as `=`, a bare function
name such as `count`, or a schema name string is **not** sufficient proof
that the runtime will invoke a trusted builtin. PostgreSQL allows user-defined
operators, functions, aggregates, and casts that can shadow or overload
builtins depending on `search_path`, type resolution, and `pg_catalog`
contents. Without proving the **actual resolved implementation identity**,
classification and admission must remain `indeterminate`.

**P1 safety gap (corrected):** treating `pg_catalog` + `provolatile in ('i','s')`
as a general trust predicate is **unsafe**. PostgreSQL semantics:

- `STABLE` guarantees the function does not modify the database; it does **not**
  guarantee the function does not read other tables, configuration, role
  context, or other hidden sources.
- `IMMUTABLE` is a planner/catalog claim; PostgreSQL does not forcibly prevent
  table reads from an incorrectly labeled immutable function.
- Query Access strict requirements promise coverage of all permission-bearing
  table/column reads. Any effect that can read a relation (or other hidden
  source) without that object appearing in extracted requirements **must not**
  be auto-admitted via volatility alone.

Therefore catalog binding (OID, namespace, operand types, volatility) is a
**necessary** fact set for identity resolution — never a sufficient condition
for `Trusted`.

**P2 safety gap (unqualified base relations):** when a PostgreSQL query
contains unqualified base relations (e.g., `SELECT id FROM users` without a
schema qualifier), the resolver must not fill in `DefaultSchema` to produce
physical table/column requirements. PostgreSQL `search_path` is
session-controlled and runtime-dependent; the offline analyzer cannot know
which schema `users` resolves to at execution time. Forging
`public.users.id` requirements from an unqualified `users` reference would
give callers a false permission proof that might resolve to a different
schema at runtime.

Fix: unqualified PostgreSQL base relations in the trusted path are marked
`Unbound` and excluded from the resolution state (`nameMap`, `aliasMap`,
`relationOrder`). Resolution, wildcard expansion, and requirement generation
skip unbound references. A bounded `unqualified_relation` indeterminate
requirement is added instead. When a query mixes qualified and unqualified
references to the same table name (e.g., `public.users p JOIN users u`), the
parser resolves aliases to table names, so both columns produce `Table: "users"`.
The resolver uses `nameMap` to distinguish: if a qualified entry exists,
resolution proceeds through it; if only unbound entries exist, resolution is
skipped entirely.

## Decision

### 1. Public admission distribution may change under proven identity

When (and only when) every effect-bearing construct in the accepted read-only
subset has a **catalog-resolved identity that is listed in the application-
maintained trusted-effect manifest**, PostgreSQL analysis may return:

- `read_classification: read_only`
- `admission: admissible` (subject to existing unresolved/wildcard/ambiguity
  rules and mode requirements)

Unknown, unproven, ambiguous, metadata-failed, or non-manifest effects remain
`indeterminate`. Write/lock/file/session mutation paths remain `not_read_only`
→ `rejected` as today.

This is a **public behavior change** relative to v0.380.0's de facto
"PostgreSQL never admissible" distribution. Callers that treated every
PostgreSQL `indeterminate` as permanent will see some queries become
`admissible`. Callers that already fail-closed on `indeterminate` remain safe.

### 2. Identity proof, not syntax allowlists

Promotion from `indeterminate` → `read_only` for effect-bearing constructs
requires a **Catalog / Effect Identity Resolver** (extension of today's
relation-only `SchemaResolver`) that returns **facts only**, at minimum:

| Construct | Must resolve (facts) |
|-----------|----------------------|
| Operator (`A_Expr`) | Resolved operator OID; owning namespace; operand types used in resolution; underlying function OID (`oprcode`); implementation namespace; volatility |
| Function / aggregate (`FuncCall`) | Resolved function OID; owning namespace; argument type list used in resolution; volatility; aggregate vs plain function distinguished |
| Cast (`TypeCast`) | Resolved cast path (source type OID → target type OID); implementation function OID if any; cast method |

The resolver **must not** claim `Trusted`. Trust is decided only by
application/domain policy against a **version-scoped, per-item audited
effect identity manifest**.

**Not sufficient alone (never trust roots):**

- operator spelling `=`
- function name `count` / `COUNT`
- schema label strings without OID binding
- "looks like a builtin" heuristics
- `pg_catalog` namespace alone
- `STABLE` or `IMMUTABLE` volatility alone
- `pg_catalog` + `provolatile in ('i','s')` as a universal predicate

If type inference cannot produce a unique catalog match, or the match is not
in the manifest, or the resolver errors / returns unknown → **do not promote**.

### 3. Fail-closed defaults

- **No effect-identity resolver:** PostgreSQL behavior remains as foundation
  (effect presence → indeterminate; no lift). MySQL/TiDB keep current
  admissible set; do **not** regress MySQL operator-bearing admissible cases.
- **Resolver returns unknown / no rows:** keep `indeterminate` with a bounded
  reason code (new codes as needed, e.g. `unproven_operator_effect`,
  `unproven_function_effect`, `unproven_cast_effect`, `identity_lookup_failed`).
- **Resolver transport / query error:** treat as indeterminate (same as
  unresolved metadata), never as `read_only` or `admissible`. Do not echo
  connection strings, credentials, SQL text, or driver internals in public
  results.
- **Ambiguity / multi-match / requires non-trivial coercion:** indeterminate.
- **Resolver returns `pg_catalog` + stable/immutable identity not in manifest:**
  indeterminate (fail closed). Volatility and catalog membership are facts,
  not trust.
- **Wildcard not fully expanded / incomplete relation metadata:** indeterminate
  (unchanged).

### 4. Trust policy (manifest-gated)

#### 4.1 Catalog facts are necessary, not sufficient

Runtime OID binding, unique identity resolution, namespace, operand/argument
types, and volatility are **necessary preconditions** for evaluating an effect
against the trust policy. They **do not** equal `Trusted`.

#### 4.2 Phase 1 trusted-effect identity manifest

Phase 1 may allow only effects listed in an **application-maintained**
manifest that is:

1. **Bounded** — finite, explicit identity entries (OID and/or unique
   `(namespace, name, arg type OIDs)` for a documented PostgreSQL version
   range); not an open-ended class.
2. **Version-scoped** — each entry (or the whole manifest) states the
   PostgreSQL major version range for which the audit holds.
3. **Per-item semantically audited** — each allowed entry must document why
   it is safe for Query Access admission, not merely that it is a builtin.

For every allowed entry, the audit must prove all of:

1. **Unique data dependency is extracted AST parameters** — the effect's
   observable data inputs are only the already-extracted operands/arguments
   from the statement AST (plus fixed type identity), not hidden sources.
2. **No hidden reads** — the effect does not read relations, server
   configuration, role/session authorization state, files, network, or other
   non-AST sources in a way that could carry permission-bearing data.
3. **Requirements completeness** — admitting the effect cannot cause strict
   requirements to omit a known physical permission-bearing table/column that
   the effect could access.

#### 4.3 Explicit non-policies

- **Must not** use syntax/name allowlists (`=`, `count`, schema names) as proof.
- **Must not** generically allow all `STABLE` functions.
- **Must not** generically allow all `IMMUTABLE` functions.
- **Must not** generically allow all `pg_catalog` functions/operators/casts.

#### 4.4 Candidates vs automatic trust

`COUNT`, basic comparison operators, and logical operators are **candidates
only** when each has:

- an explicit identity manifest entry (or closed set of entries),
- written semantic rationale meeting §4.2,
- version support policy,
- test matrix coverage (positive + adversarial).

If any of those cannot be proven, keep `indeterminate`.

#### 4.5 Always indeterminate (non-exhaustive)

- `current_setting` and similar GUC/session readers
- `pg_get_*` catalog pretty-printers / metadata helpers that read system state
- any builtin that reads hidden context or metadata beyond AST operands
- any `pg_catalog` stable/immutable function/operator **not** in the manifest
- user-defined operators, functions, aggregates, casts
- function-backed casts by default

#### 4.6 Casts

- **Function-backed casts** (`castfunc` present / `castmethod` function):
  default **unsupported** → `indeterminate`.
- Phase 1 may consider only **binary casts with no function call** after the
  same per-item manifest audit proves they introduce no hidden reads and no
  requirements gap.
- All other casts remain `indeterminate`.

#### 4.7 Operators and aggregates

- For operators: both the operator row and `oprcode` function must resolve
  uniquely and **both** pass manifest trust (or the operator identity maps to
  a single audited implementation entry).
- User-defined operators/functions/casts in non-catalog schemas never trust.

Read-only classification remains independent of admission mode (`strict` /
`projection_only`). **Read-only does not imply admissible** when requirements
are unresolved.

### 5. Surface and privacy contracts

- SDK, CLI, and HTTP continue to call the **same** application service;
  transports must not re-implement effect logic.
- MCP still does **not** gain a query-access tool (unchanged non-goal).
- Results still omit raw SQL, normalized SQL, literals, credentials, connection
  strings, parser near-text, and catalog connection details by default.
- Relation/column identifiers remain intentional outputs.
- No `severity` field is introduced. Query access remains outside audit
  findings/`level`.

### 6. Dialect non-alignment

Do **not** relax MySQL/TiDB to "match" PostgreSQL, and do **not** force MySQL
through a weaker name-only path to invent PG parity. Optional later phases may
add MySQL identity-backed function promotion only if a MySQL-safe proof model
exists; it is not required for the first implementation cut.

Do not change MySQL/TiDB current behavior in this milestone.

## Rationale

- The foundation already defined the product scenario; the blocker is not MCP
  symmetry or a new flag, but **unproven effects**.
- PostgreSQL's overload and `search_path` model make name allowlists unsafe.
- PostgreSQL volatility classes are **not** a pure-read permission model:
  STABLE/IMMUTABLE do not prove "no table or hidden-context reads."
- Query Access strict requirements cover permission-bearing physical sources;
  auto-trusting any stable/immutable catalog function would violate that
  completeness promise.
- `pg_query_go` parse trees expose operator/function **names and structure**,
  not OIDs; catalog resolution is mandatory for identity facts, and a
  **bounded audited manifest** is mandatory for trust.
- Keeping fail-closed on missing proof and non-manifest identities preserves
  the foundation's security posture while unlocking only demonstrably safe
  shapes.

## Public Contract

After this decision is **Accepted** and implemented, consumers may rely on:

1. PostgreSQL queries that previously were always `indeterminate` may become
   `read_only` + `admissible` when every effect is catalog-resolved **and**
   listed in the trusted-effect manifest, and all existing admission
   preconditions hold.
2. Absence of proof, or resolved identity outside the manifest, never yields
   `admissible`.
3. `pg_catalog` + stable/immutable without a manifest entry remains
   `indeterminate`.
4. New bounded reason codes may appear for unproven effects; they are additive
   machine identifiers, not free-text SQL.
5. MySQL/TiDB admissible cases that exist today remain admissible unless a
   separate accepted decision says otherwise.
6. MCP tool list remains without query-access.
7. No `severity` field; no raw SQL / credential leakage in the public result.
8. `admissible` means the static analysis found a complete requirement set
   for the observed metadata and session resolution context. It does **not**
   authorize execution, evaluate grants, or guarantee that a later query
   execution uses the same database snapshot, catalog state, or search_path.

Exact Go type names for the extended resolver and manifest remain
implementation details until the implementation plan locks them; the **proof
and trust requirements** above are the contract principles.

## Deferred / Out Of Scope

- MCP query-access tool
- Grant evaluation, authentication, session authorization
- RLS evaluation, masking, SQL rewrite
- View definition expansion to base tables
- Dynamic SQL inside routines
- Full planner-grade type coercion graph
- Generic "SQL purity theorem" for all expressions
- Parser upgrade of unrelated DDL forms
- Name-only allowlists as a production trust root
- Generic volatility-based (`i|s`) allowlists as a production trust root
- Changing audit rule evaluation or audit verdicts
- Changing MySQL/TiDB admission behavior

## Verification Evidence

### Design-time / foundation characterization

- Current PG corpus: 22/22 `admission: indeterminate`.
- Current code: `pgContainsOperatorExpr` returns true on any `A_Expr`/`TypeCast`;
  `pgContainsFunctionCallInExpr` on any `FuncCall`; no OID capture.
- `reclassifyAfterResolution` hard-stops PostgreSQL lifts.
- Relation `SchemaResolver` queries `pg_class`/`pg_attribute` only; no
  `pg_operator` / `pg_proc` / `pg_cast`.
- Trust model: catalog facts + audited manifest (not volatility class).

### T2 — Effect identity manifest feasibility (2026-07-12)

**Conclusion: Proceed** (bounded, version-scoped, per-item audited manifest is
maintainable for a minimal phase-1 set). Not Accepted for product behavior yet;
no production implementation in T2.

#### Research environment

| Source | Finding |
|--------|---------|
| Repo Docker E2E | `docker/pg-e2e-compose.yaml` → `postgres:17` only |
| Metadata product claim | CHANGELOG: live metadata against PostgreSQL **12+** |
| Parser | `pg_query_go/v6` / libpg_query **17**; PostgreSQL **18** parser forms deferred |
| CI/workflows | Release smoke uses dialect postgresql offline; live metadata e2e uses PG17 compose |
| Ephemeral probes (not committed) | Docker `postgres:14`, `postgres:16`, `postgres:17` catalog queries under `/tmp` |
| Official docs | OID assignment policy; function volatility categories (current docs) |

**Evidence gaps (must not be guessed away):**

- PostgreSQL **12, 13, 15** candidate OIDs were **not** probed in this session.
- Phase-1 **product claim for effect-identity promotion** is therefore limited to
  majors **with probe evidence**: **14, 16, 17** (and CI primary **17**).
- Extending the claim to 12–13 or 15 requires the same identity-tuple probe, not
  extrapolation alone.
- PostgreSQL **18** remains out of phase-1 parse/promotion scope until parser
  support lands.

#### OID stability (probed subset)

- Official: manually assigned catalog OIDs, once released, are not renumbered;
  auto-assigned bootstrap OIDs (approx. 10000–16383) are **not** stable across
  installations.
- Ephemeral probe: for the closed candidate set (core scalar types; comparison
  ops on those types; `count(*)` / `count(anyelement)`), operator OID,
  implementation function OID, type OID, and aggregate OID were **identical**
  across PostgreSQL **14.23**, **16.14**, and **17.10**.
- Sample stable facts (all three majors): `int4` type OID 23; `=`(int4,int4)
  operator OID 96 / impl `int4eq` 65; `=`(text,text) 98 / `texteq` 67;
  `count(*)` 2803; `count(anyelement)` 2147; `pg_catalog` namespace OID 11.
- `pg_cast` row OIDs observed for binary casts were in the **≥10000** range →
  **must not** be primary manifest keys; key binary casts by
  `(castsource, casttarget, castmethod)` (and `castfunc = 0`) instead.

#### Catalog fields usable for runtime identity proof

- **Operator:** `pg_operator.oid`, `oprnamespace`, `oprname`, `oprleft`,
  `oprright`, `oprresult`, `oprcode` (+ join `pg_proc` for impl volatility/kind).
- **Function:** `pg_proc.oid`, `pronamespace`, `proname`, `proargtypes`,
  `prorettype`, `provolatile`, `prokind`.
- **Aggregate:** `pg_aggregate.aggfnoid` (+ `pg_proc` signature), `aggkind`,
  transition/final fn OIDs for audit notes (not name trust).
- **Cast:** `castsource`, `casttarget`, `castfunc`, `castmethod` (`b`/`f`/`i`),
  `castcontext`. Prefer type-pair + method over cast row OID.
- **Namespace:** `pg_namespace.oid` / `nspname` (fact; trust still needs
  manifest).

#### Minimal identity tuples (manifest keys)

- Operator: `(pg_major_range, operator_oid)` **and** verify at resolve time
  `(oprnamespace, oprname, oprleft, oprright, oprcode)` match the entry; or
  equivalently key by canonical signature
  `(namespace_oid|name, oprname, left_type_oid, right_type_oid)` with unique
  match + expected `oprcode`.
- Function/aggregate: `(pg_major_range, function_oid)` **and** verify
  `(pronamespace, proname, proargtypes, prokind)`.
- Cast (if any): `(pg_major_range, castsource, casttarget, castmethod)` with
  `castfunc = 0` for binary-only phase-1 candidates.
- **Never** key trust by operator spelling, bare function name, or schema
  string alone.

#### Volatility (fact only)

Per official Function Volatility Categories:

- `STABLE`: cannot modify DB; same result for same args within a statement;
  **may** `SELECT` from tables (snapshot-fixed).
- `IMMUTABLE`: claim of no DB lookup / same forever; **not enforced** against
  table reads; mislabeling is possible.
- Therefore `provolatile` is recorded as a **fact**, never as Trusted.

#### Phase-1 candidate ledger (audit sketch)

Eligible for later manifest entries **only** after identity resolve + tests:

1. **Structural `BoolExpr` (AND/OR/NOT)** — not a catalog effect; pure AST
   control structure. Trust as structural when child effects are trusted or
   absent. No OID.
2. **Closed comparison operators** on same-type pairs for
   `{bool,int2,int4,int8,float4,float8,numeric,text,oid}` ×
   `{=,<>,<,>,<=,>=}` — 54 identities on PG17; all probed impls `provolatile=i`,
   `prokind=f`, namespace `pg_catalog`. Data deps = left/right operands only;
   no relation/GUC/role/file/network. Requirements completeness preserved
   (operands already extracted as column/const).
3. **`count(*)` (OID 2803)** and **`count(anyelement)` (OID 2147)** —
   aggregate over already-extracted FROM/JOIN row sources / argument
   expression; does not open hidden relations. Must still resolve unique
   aggregate identity (not name `count`).

**Deferred / not in minimal phase-1 promote set:**

- **Binary casts:** few core pairs are `castmethod=b` (`int4`↔`oid`,
  `text`/`varchar`/`bpchar` family). Cast row OIDs unstable. Common numeric
  casts are **function-backed** → remain indeterminate. Phase 1 may omit casts
  entirely (recommended default) or add type-pair binary entries later with
  extra tests.
- **varchar same-type operators:** not in the closed same-type op set used
  here; typically require coercion to text → fail-closed without coercion
  graph.
- Cross-type comparisons requiring coercion → indeterminate.

#### Rejected ledger (always indeterminate unless a later decision re-audits)

- Any `pg_catalog` operator/function/cast **not** in the manifest
- `current_setting` / `set_config` (session/GUC) — probed present, `STABLE`
- `pg_get_*` metadata helpers — probed sample set; typically `STABLE`/`VOLATILE`
- `pg_read_file`, `pg_ls_dir`, and other file/OS readers
- User-defined operators/functions/aggregates/casts (any volatility)
- Function-backed casts (`castmethod=f` / non-zero `castfunc`)
- Volatile catalog functions
- Name/schema/spelling allowlists; generic `i|s` volatility allowlists

#### Privileges / context / no-leak

- Identity resolution needs SELECT on `pg_catalog` catalogs (same class of
  privilege as existing relation `SchemaResolver` pg_class/pg_attribute reads).
- Caller must lock execution-equivalent **search_path / database / role**
  context; prefer resolve only inside `pg_catalog` for trust evaluation.
- Optional: frozen metadata snapshot of identity facts — still evaluated
  against the application manifest.
- Public result contract must not include DSN, credentials, catalog SQL text,
  raw user SQL, literals, or `severity`. Design docs may record abstract
  field names and non-sensitive OID statistics only.

#### Feasibility vs kill criteria

| Kill criterion | T2 assessment |
|----------------|---------------|
| Bounded type inference impossible for matrix | Phase-1 matrix uses **exact same-type** operands only; no full planner |
| Unique identity requires accepting search_path shadowing | Resolve locked to `pg_catalog` + unique type match; multi-match → indeterminate |
| Catalog access forces secret leak | Existing scrub patterns apply; no public catalog SQL |
| Collapses to name allowlist | No — OID/signature + manifest |
| Collapses to volatility class allowlist | No — per-item manifest; volatility is fact |
| Unmaintainable open set | Closed 54-op + 2-count + structural bool; version range 14–17 probed |

**Proceed conditions for later tasks:** implement only this closed set (or a
subset); fail closed outside it; re-probe before expanding major range.

Planned gates after implementation: corpus matrix (safe positives + adversarial
negatives including non-manifest `pg_catalog` stable, `current_setting`,
`pg_get_*`, function-backed cast), unit tests for identity resolver (facts
only), trust policy manifest tests, cross-surface SDK/CLI/HTTP parity, no-leak
tests, `make query-access-corpus-gates`, dialect-tagged PostgreSQL tests.

### T3 — Effect identity characterization (tests + corpus only)

**Date:** 2026-07-12
**Status remains:** `Proposed` (characterization does **not** accept product
promotion behavior).

**What T3 locks (evidence only; not supported candidate effects):**

1. **Structural `BoolExpr` (AND/OR/NOT)** is pure AST control structure. Alone
   (column refs only) it does **not** force `indeterminate` classification and
   is **not** catalog identity proof. Wrapping a comparison candidate still
   freezes both classification and admission as `indeterminate`.
2. **T2 candidate comparison matrix** representative forms for
   `=`, `<>`, `<`, `>`, `<=`, `>=` (int/text/bool samples, including
   `OPERATOR(pg_catalog.=)`) remain `read_classification=indeterminate` and
   `admission=indeterminate` with and without relation `SchemaResolver`.
3. **`COUNT(*)` / `COUNT(column)`** remain indeterminate; only a future
   manifest entry + identity resolver + proof engine may promote.
4. **Type-resolution / coercion-looking comparisons** (int=`'x'`, float mix,
   column-column) remain indeterminate — unique OID is not guessed.
5. **Rejected ledger** remains fail-closed indeterminate:
   `current_setting`, `set_config`, `pg_get_*`, `pg_read_file` / `pg_ls_dir`,
   UDF-looking / non-`pg_catalog` schema-qualified functions, function-backed
   cast / `TypeCast` / `CAST(...)`, non-manifest catalog-looking builtins
   (`now`, `version`).
6. **Unknown / coercion / multi-match shapes** (params, `IS NOT DISTINCT FROM`,
   `||`) remain indeterminate.
7. **No resolver, resolver error, incomplete metadata** keep effect-bearing
   PostgreSQL queries indeterminate. Public results/errors must not include
   SQL text, literals, credentials, connection strings, catalog query text, or
   `severity`.
8. **`reclassifyAfterResolution` never lifts PostgreSQL** out of
   `indeterminate` (foundation hard-stop). MySQL safe-lift path is preserved.
9. **MySQL/TiDB** operator-bearing SELECT (`WHERE` comparisons) remains
   `read_only` + `admissible` (no cross-dialect regression).

**Explicit non-claims of T3:**

- Tests characterize current behavior only. They do **not** mean candidate
  effects are supported, trusted, or admissible.
- PostgreSQL promotion remains **forbidden** until later tasks complete:
  effect-identity resolver (facts only), version-scoped audited manifest, and
  proof engine / admission promotion wiring.
- Rejected-ledger forms continue fail-closed even if catalog-stable/immutable.

**Artifacts:**

- Application: `effect_identity_characterization_postgresql_tag_test.go`,
  `effect_identity_mysql_tidb_regression_test.go`
- Parser AST: `query_access_effect_identity_postgresql_tag_test.go`
- Corpus (representative): `select_structural_bool_and`, `select_comparison_ne`,
  `select_comparison_ge`, `select_count_star`, `select_count_column`,
  `select_current_setting`, `select_udf_like`, `select_type_cast`
  (plus existing `select_with_where` / function / join fixtures)

### T4 — Bounded unproven-effect reason codes (2026-07-12)

**Status remains:** `Proposed` (reason codes explain why PostgreSQL stays
`indeterminate`; they do **not** accept product promotion behavior).

**What T4 delivers:**

1. **Additive domain reason codes** (machine identifiers only):
   - `unproven_operator_effect`
   - `unproven_function_effect` (functions and aggregates)
   - `unproven_cast_effect`
   - `identity_resolver_unavailable`
   - `identity_unknown`
   - `identity_lookup_failed`
   - `identity_ambiguous`
   - `identity_coercion_gap`
2. **Assignment path (presence-only):** PostgreSQL extract walks `A_Expr` /
   `FuncCall` / `TypeCast` and emits the corresponding `unproven_*` codes.
   Codes never include operator spellings, function names, cast targets, OIDs,
   SQL text, literals, credentials, or driver errors.
3. **Identity failure mapping:** `ReasonForIdentityFailure` accepts only bounded
   `IdentityFailure` categories; free-text / error strings cannot be injected
   as trusted reasons. Underlying errors are discarded when attaching codes.
4. **Determinism:** `NormalizeReasonCodes` deduplicates and sorts; same SQL
   yields the same reason order across analyses.
5. **Admission unchanged:** candidate and rejected effect-bearing PostgreSQL
   queries remain `read_classification=indeterminate` and
   `admission=indeterminate`. Mode (`strict` / `projection_only`) does not
   change classification. MySQL/TiDB operator-bearing admissible cases do not
   receive `unproven_*` / `identity_*` codes and do not regress.
6. **Cross-surface:** SDK, CLI, and HTTP continue to passthrough the single
   application result; no per-surface branching; no `severity`; no MCP tool.
7. **Corpus:** PostgreSQL expected fixtures record reason ids only (no effect
   text).

**Explicit non-claims of T4:**

- No effect identity resolver implementation
- No trusted-effect manifest / trust policy evaluation
- No proof engine
- No PostgreSQL admission promotion (`reclassifyAfterResolution` still hard-stops PG)
- No `effect_not_in_trust_manifest` code (optional; deferred until manifest wiring)

**Artifacts:**

- Domain: `model.go` / `normalize.go` (+ tests, package README)
- Parser: `query_access.go` presence flags → reason codes
- Application: `extract_postgresql.go` mapping; `service.go` normalize
- Tests: application unproven-effect + MySQL/TiDB regression; parser reasons;
  SDK/CLI/HTTP postgresql-tag passthrough
- Corpus: `testdata/query-access/postgresql/*.expected.yaml` reason ids

**T4 follow-up — complete effect traversal (pre-T5 gate):**

The initial T4 walker only visited target/WHERE/FROM/HAVING/GROUP/ORDER and
missed executable expression positions (LIMIT/OFFSET, VALUES, window
partition/order/frame, aggregate FILTER, DISTINCT ON, etc.). Smoke repro:
`SELECT id FROM users LIMIT length('a')` and `VALUES (length('a'))` returned
`read_only` with empty reasons (admission stayed seeded indeterminate without a
resolver, so not an immediate promotion bug, but it violated the explain-
unproven-effect contract and would be unsafe if a later proof engine reused the
partial walker).

Fix: one auditable `collectSelectEffects` / `collectNodeEffects`
traversal covering all SelectStmt expression fields and nested FuncCall
(args/ORDER/FILTER/OVER), WindowDef, List/VALUES, RangeFunction, etc.
Classification uses the same traversal (`selectHasUnprovenEffect`). Missed
positions now emit `unproven_*` only — never proven/admissible.

### T5 — Effect candidate extraction (internal only; 2026-07-12)

**Status remains:** `Proposed` (candidates are resolver **inputs**, not trust
roots; public admission is still not promoted).

**What T5 delivers:**

1. **Internal `EffectCandidate` facts** on parser `QueryAccessFacts` and
   application `QueryAccessResult` (not on `domain.Result`):
   - `Kind`: operator | function | cast
   - stable traversal `Ordinal` (0-based, contiguous, deterministic)
   - `NamePath` / cast `TargetTypePath` (internal spelling only)
   - `ExplicitSchema`, `Arity`, `OperandKinds` (column/const/param/star/expr/…)
   - structural flags: `IsAggregate`, `HasWindow`, `HasFilter`
2. **Collected by the same complete effect traversal** as T4 reasons
   (LIMIT/OFFSET, VALUES, window, FILTER, DISTINCT ON, nested CTE/subquery).
3. **Structural BoolExpr is not a catalog candidate** (children only).
4. **Public behavior unchanged:**
   - effect-bearing PostgreSQL still `indeterminate` + `indeterminate`
   - public reasons remain bounded `unproven_*` only
   - no candidate names/paths/OIDs/SQL/literals/`severity` on SDK/CLI/HTTP JSON
   - `QueryAccessRequest` has no candidate/trust injection fields
   - MySQL/TiDB behavior and extraction unchanged
5. **Explicit non-claims:** no catalog identity resolver, no manifest trust
   policy, no proof engine, no admission promotion.

**Artifacts:**

- Parser: `query_access_effect_candidates.go` + extraction wired in
  `query_access.go`
- Application: `EffectCandidate` on `QueryAccessResult` only; map from parser
- Tests: parser candidate extraction; application public-output freeze;
  SDK/CLI/HTTP no-candidate-field checks
- Package READMEs note internal-only/untrusted contract

### T6 — Effect identity resolver contract (facts only; 2026-07-12)

**Status remains:** `Proposed` (contract only; no catalog adapter, no trust
policy, no admission promotion).

**What T6 delivers:**

1. **Domain `IdentityStatus`** (bounded per-candidate outcomes):
   `resolved`, `unknown`, `ambiguous`, `coercion_gap`, `lookup_failed`,
   `unavailable`. Free-text is invalid. Fail-closed for all non-`resolved`.
   Mapping helpers: `IdentityStatusToFailure`, `ReasonForIdentityStatus`
   (`lookup_failed` → `IdentityFailureError` → `identity_lookup_failed`).
2. **Application `EffectIdentityResolver`** batch contract:
   - Input: `EffectIdentityRequest` keyed by candidate ordinal (internal
     `EffectCandidate` facts + optional operand type OID hints).
   - Output: `EffectIdentityBatch` / `EffectIdentityItem` with bounded
     `IdentityStatus` and optional `EffectIdentityFacts` (OIDs, namespace,
     operand/result types, implementation OID, volatility, cast method,
     internal `CanonicalSignature`).
   - **No** `Trusted`, admission, reason free-text, driver error text, catalog
     SQL, DSN, or candidate-name injection into domain Result / public JSON.
3. **Batch semantics:** unique ordinals on request; deterministic ascending
   ordinal order; partial failure fills missing ordinals as `unavailable`
   (`CompleteEffectIdentityBatch`); free-text status rewritten to
   `lookup_failed`; context cancellation is a batch-level error (not a
   per-item status).
4. **ColumnSchema.TypeOID** optional fact field (zero = unknown). T6 does not
   populate it via catalog queries.
5. **Public surface unchanged:** no `EffectIdentityResolver` on application or
   public SDK `QueryAccessRequest`; CLI/HTTP JSON schemas unchanged;
   `Service.Analyze` does not invoke the resolver; PostgreSQL remains
   `indeterminate` + `unproven_*` only (no `identity_*` attached by Analyze).
6. **Explicit non-claims:** no PostgreSQL `pg_catalog` identity SQL (T7); no
   manifest trust judgment or admission promotion (T8); no public resolver
   injection until a complete end-to-end path exists.

**Artifacts:**

- Domain: `IdentityStatus` + mapping helpers in `model.go` / `normalize.go`
- Application: `identity_resolver.go` + contract tests; `ColumnSchema.TypeOID`
- Docs: package READMEs; this T6 Evidence; implementation plan/OMO mark T6
  facts-only / T7 adapter / T8 promotion

### T6 P1 amendment — execution resolution context (2026-07-12)

**Status remains:** `Proposed`. **Blocking for T7:** catalog adapter work must
not start until this contract is present (now) and honored by the adapter.

**Problem:** T2 forbids proving identity from function/operator spelling or
`pg_catalog` name allowlists. Real PostgreSQL resolution for unqualified
effects (`count(*)`, `id = 1`) depends on overload ranking, argument types,
and `search_path` / session state. The initial T6 request carried only
dialect + candidates + optional type OIDs — insufficient to prove the runtime
identity. A T7 that queried `pg_catalog.count` / `pg_catalog.=` by name would
regress into a forbidden allowlist. No product vuln yet (`Analyze` does not
call the resolver), but the contract gap blocked safe T7.

**What the amendment delivers:**

1. **`EffectIdentityResolutionContext`** (internal-only on
   `EffectIdentityRequest.Resolution`):
   - Phase-1 promotion-ready session complete requires **all non-zero**:
     `Bound`, `SessionBinding`, `PathEpoch`, `DatabaseOID`, `RoleOID`,
     `ServerVersionNum`
   - ordered `NamespaceSearchOIDs` additionally required for **unqualified**
   - **Never** on `domain.Result`, SDK/CLI/HTTP JSON, reason codes, or public
     errors (no DSN, password, `search_path=` text, catalog SQL).
2. **Phase-1 resolution policy (normative):**
   - Incomplete session context → **all** candidates **`unavailable`**
     (including explicit schema). OIDs are database-local; cross-db/major
     facts must not reach T8.
   - **Unqualified** also needs non-empty search path; otherwise unavailable
     (no `pg_catalog.<name>` guess).
   - **Explicit schema** skips search_path ranking only; still requires full
     session/database/role/server/epoch binding.
   - Resolved facts must carry matching `DatabaseOID` + `ServerVersionNum`
     pins (`StampFactsFromResolution`).
   - **TOCTOU:** live session mismatch (binding/epoch/db/role/version) strips
     **all** candidates; search_path-only mismatch strips unqualified only.
3. **Helpers:** `ResolutionContextSessionComplete`,
   `ResolutionContextSessionCompatible`, `ResolutionContextSearchPathCompatible`,
   `ResolutionContextsCompatible`, gates, stamp helper.
4. **Tests:** zero-field contexts; role/database/server-version/epoch/session
   live mismatch strips explicit too; path-only mismatch keeps explicit;
   unpinned/wrong-db/version facts discarded; shadowing/overload/no public leak.

**T7 obligation:** on one controlled session — read live context → identity
lookup → re-read live context → `GateIdentityBatchAgainstLiveContext` → only
then hand facts to T8. Never resolve unqualified without complete binding.

### T6 P1b — session/database/role/version completeness (2026-07-13)

**Status remains:** `Proposed`. **Still blocking for T7.**

Earlier P1 required Bound + SessionBinding + search path only, treated
DatabaseOID/RoleOID as optional, and let explicit-schema facts survive live
session mismatch. That was insufficient: the same opaque binding could drift
role/database without a correct epoch update, and database-local OIDs could
escape to T8.

**Hardening:**

- Bound contexts require non-zero PathEpoch, DatabaseOID, RoleOID,
  ServerVersionNum (Validate rejects partial Bound).
- Compatibility never treats zero as "not asserted".
- Explicit schema cannot skip session/db/role/server checks.
- Live role/db/version mismatch → strip all facts including explicit.

### T7 — PostgreSQL catalog effect-identity adapter (facts only; 2026-07-13)

**Status remains:** `Proposed` (catalog facts only; **no** Analyze wiring, **no**
manifest trust, **no** admission promotion).

**What T7 delivers:**

1. **`PinnedSession`** wrapping a single `*sql.Conn` (not multi-connection
   `*sql.DB` lookups). `PinSession` / `NewPinnedSessionFromConn` are the only
   production entry points into the adapter.
2. **`EffectIdentityAdapter`** implementing application
   `EffectIdentityResolver`:
   - capture live context (database OID, role OID, `server_version_num`,
     backend binding, ordered `current_schemas(true)` namespace OIDs)
   - exact catalog lookups for operator / function / cast
   - re-capture live context
   - `GateIdentityBatchAgainstLiveContext` (TOCTOU)
3. **Exact-match policy:** requires operand/argument type OIDs from the
   request; param/star/expr/unknown kinds → `coercion_gap`; multi-match →
   `ambiguous`; zero match → `unknown`; transport errors → `lookup_failed`
   without leaking driver text onto public Result.
4. **No fact cache** across SessionBinding/DatabaseOID/RoleOID/
   ServerVersionNum/PathEpoch. Resolved facts always
   `StampFactsFromResolution`.
5. **Public behavior unchanged:** adapter is **not** called from
   `Service.Analyze`; PostgreSQL remains indeterminate + `unproven_*`;
   SDK/CLI/HTTP schema unchanged; MySQL/TiDB untouched.
6. **Runtime evidence:** unit tests with fake pinned catalog; optional PG17
   Docker integration (`-tags postgresql,integration`) against
   `docker/pg-e2e-compose.yaml` (`postgres:17`). T2 research range 14–17 is
   **not** claimed as multi-version CI for this adapter.

**Artifacts:**

- `internal/infrastructure/metadata/postgresql/effect_identity_session.go`
- `internal/infrastructure/metadata/postgresql/effect_identity_resolver.go`
- unit + optional integration tests; package README

**Explicit non-claims:** no TrustPolicy / manifest evaluation (T8); no
admission lift; no public resolver injection.

### T8 — Manifest proof engine and admission promotion (2026-07-13)

**Status remains:** `Proposed` (manifest proof implemented; PG17 promotion
limited to exact manifest match under controlled session).

**What T8 delivers:**

1. **`TrustPolicy`** — sole path to `Trusted` for PostgreSQL admission.
   `IsTrusted(batch, serverVersionNum)` evaluates resolved facts against the
   versioned manifest. Returns `TrustDecision` (all_proven | has_unproven |
   has_unknown | empty).
2. **`PG17Manifest`** — immutable, compile-time owned, versioned manifest
   containing T2 ledger's audited minimum closed set:
   - 54 comparison operators (9 types × 6 ops) with exact OIDs
   - 2 aggregates: `count(*)` (OID 2803), `count("any")` (OID 2147)
   - All entries: `provolatile=i`, `pg_catalog` namespace (OID 11)
   - Schema version `1.0`, PostgreSQL major range 17–17
   - Deterministic SHA-256 hash of entries
3. **`NewTrustedService()`** — internal constructor that bundles
   `ControlledEffectIdentityResolver`, `TrustPolicy`, and `SchemaResolver`. Not
   exposed on public SDK/CLI/HTTP request schemas. The resolver must satisfy the
   narrower `ControlledEffectIdentityResolver` contract (not generic
   `EffectIdentityResolver`) so the application can capture and validate
   execution-bound context before resolution.
4. **`reclassifyAfterResolution`** — PG hard-stop replaced with manifest-gated
   promotion. Only `TrustDecisionAllProven` allows `read_only` classification.
   Without proof or with any unproven/unknown, remains `indeterminate`.
5. **Manifest proof flow:**
   - Extract effect candidates (T5)
   - Resolve identities via `EffectIdentityResolver` (T7)
   - Apply `TrustPolicy.IsTrusted()` against `PG17Manifest`
   - If all_proven: classification = `read_only`, admission = `admissible`
   - Otherwise: fail-closed to `indeterminate`
6. **Trust boundaries preserved:**
   - Trust roots: exact catalog identity facts, complete session binding,
     second live gate, versioned manifest exact match
   - Forbidden: syntax/names/schema/provolatile/castmethod/OID ranges
   - Resolver returns facts only; never Trusted/admission/free-text
   - No public leak of resolver/session/OID/manifest/SQL/literals

**Manifest entries (Phase-1):**

| Kind | ObjectOID | NamespaceOID | CanonicalSignature | Audit Notes |
|------|-----------|--------------|-------------------|-------------|
| operator | 91 | 11 | `pg_catalog.=(16,16)` | bool = bool; impl booleq |
| operator | 96 | 11 | `pg_catalog.=(23,23)` | int4 = int4; impl int4eq |
| operator | 98 | 11 | `pg_catalog.=(25,25)` | text = text; impl texteq |
| ... | ... | ... | ... | 54 total comparison operators |
| function | 2803 | 11 | `pg_catalog.count()` | count(*) aggregate |
| function | 2147 | 11 | `pg_catalog.count(2276)` | count(anyelement); arg=2276 |

**Fail-closed conditions (remain indeterminate):**

- No trusted bundle (default Service)
- Resolver error, timeout, cancellation
- Unresolved/ambiguous/coercion_gap/lookup_failed/unavailable items
- Manifest miss (resolved but not in manifest)
- Version mismatch (facts vs manifest range)
- Context drift (session/db/role/version mismatch)
- Missing type OIDs, parameters, NULL literals, coercion required
- Multi-match, unknown function/operator/cast
- Non-manifest pg_catalog stable/immutable functions
- UDFs, current_setting, pg_get_*, file functions
- Function-backed casts, binary casts (Phase-1 default)
- Missing table/column, ambiguity, wildcard expansion failure

**Public behavior:**

- SDK/CLI/HTTP: identical application results for same input
- No resolver/session/OID/candidate/manifest/DSN/SQL/literal in public JSON
- MySQL/TiDB path unchanged (no proof parameter used)
- `projection_only` preserves inference_risk behavior

**Runtime evidence:**

- `trust_policy_test.go`: 17 tests covering all trust decisions
- `trusted_service_test.go`: constructor validation, reclassify logic, deep copy
- `trusted_service_postgresql_tag_test.go`: real `Service.Analyze` promotion:
  - count(*) → read_only + admissible (controlled resolver + manifest match)
  - id = 1 → indeterminate (operator with literal → coercion_gap)
  - capture context fails → indeterminate
  - incomplete context → indeterminate
  - resolver error → indeterminate
  - manifest miss → indeterminate
  - no public JSON leak (OID/session/manifest/SQL/literal)
  - MySQL path unchanged
- All existing tests pass (398 postgresql-tagged, 213 non-tagged)
- `make query-access-corpus-gates`: PASS
- `golangci-lint`: clean
- `go vet` / `go vet -tags postgresql`: clean

**P1/P2 fixes (2026-07-13):**

- **P1-1 (execution context binding):** `resolveAndProveEffects` now captures
  explicit execution-bound context via `ControlledEffectIdentityResolver
  .CaptureExecutionBoundContext()` before building the request. Generic
  `EffectIdentityResolver` cannot trigger promotion — only controlled
  implementations with pinned sessions satisfy the narrower contract.
- **P1-2 (type OID resolution):** `buildOperandTypeOIDs` now returns a map
  with arity-0 entries (count(*)) and defers operator type resolution.
  `hasUnresolvedTypeKind` in the T7 adapter no longer blocks arity-0 functions
  with star operand kind. `SELECT id FROM users WHERE id = 1` remains
  indeterminate (literal type unknown → coercion_gap).
- **P2-1 (deep copy):** `NewPG17Manifest()` now deep copies entries and nested
  `OperandTypeOIDs` slices to prevent mutation of the compile-time backing store.
- **P2-2 (Docker E2E):** attempted; see Docker evidence section below.

**Artifacts:**

- `internal/application/queryaccess/trust_policy.go`
- `internal/application/queryaccess/trust_manifest_pg17.go`
- `internal/application/queryaccess/trust_policy_test.go`
- `internal/application/queryaccess/trusted_service_test.go`
- `internal/application/queryaccess/trusted_service_postgresql_tag_test.go`
- Modified: `internal/application/queryaccess/identity_resolver.go`
  (added `ControlledEffectIdentityResolver` interface)
- Modified: `internal/application/queryaccess/service.go`
  (explicit execution-bound context capture, buildOperandTypeOIDs)
- Modified: `internal/infrastructure/metadata/postgresql/effect_identity_resolver.go`
  (CaptureExecutionBoundContext, hasUnresolvedTypeKind arity-0 fix)
- Modified: `internal/application/queryaccess/README.md`

**Explicit non-claims:** no execution capability token; no multi-version CI
(14–17 probe evidence only); no cast promotion; no view/RLS/rewrite proof;
no public resolver injection.

### T10 — Pipeline Closure for Unqualified Relations (2026-07-14)

**Status remains:** `Proposed`. **Implemented** — fixes the early return in `Service.Analyze` for unqualified PostgreSQL relations, ensuring every result passes through the full normalization, requirements, and validation pipeline.

**What T10 delivers:**

1. **Removed early return in `Service.Analyze`:** Previously, `service.go:113` returned the raw extraction result immediately when unqualified relations were found, bypassing 7 critical pipeline steps (metadata resolution, requirement building, sorting, and validation).

2. **Fail-closed promotion barrier:** When `hasUnqualifiedRelation` is true:
   - `ReadClassification` is forced to `Indeterminate`.
   - `Admission` is forced to `IndeterminateAdmission`.
   - `ReasonUnqualifiedRelationBlocked` is appended to reason codes.
   - Effect identity resolution (`resolveAndProveEffects`) is skipped entirely.

3. **Pipeline completion:** The result now passes through `resolveMetadata` (mapping `users` → `public.users` via `DefaultSchema`), `buildRequirements`, sorting, and `domain.ValidateResult`.

4. **New domain reason code:** `ReasonUnqualifiedRelationBlocked` (`unqualified_relation_blocked`) explains the barrier without leaking session details.

5. **Test coverage:**
   - `TestTrustedService_UnqualifiedRelationFullPipeline`: Verifies requirements are built, admission is indeterminate, and resolver is bypassed.
   - `TestTrustedService_UnqualifiedRelationStructuralQuery`: Verifies structural queries (no effects) are also blocked.
   - `TestTrustedService_UnqualifiedRelationResolverError`: Verifies the barrier holds even if the resolver fails.
   - `TestTrustedService_UnqualifiedRelationNoPublicLeak`: Verifies no OIDs/session info leak.

**Oracle/Momus Review Findings (addressed):**

- **P1 (Oracle):** Structural queries (no effect candidates) could bypass the barrier if `ReadClassification` was already `read_only`. **Fixed:** The barrier forces `Indeterminate` regardless of initial classification.
- **P1 (Momus):** Missing test for resolver failure/cancellation during the new flow. **Fixed:** `TestTrustedService_UnqualifiedRelationResolverError`.
- **P1 (Momus):** Docker E2E must wait for PostgreSQL readiness. **Noted:** `docker compose up -d --wait` used.
- **P2 (Momus):** `ReasonUnqualifiedRelationBlocked` belongs in `domain/queryaccess/model.go`. **Fixed.**

**Cross-surface consistency:**

- CLI, HTTP, and SDK continue to use the same application result.
- No public injection of trusted resolver on public surfaces.
- MySQL/TiDB behavior unchanged.

**Verification evidence:**

- `go test ./internal/application/queryaccess/... -count=1`: 213 passed
- `go test -tags postgresql ./internal/application/queryaccess/... -count=1`: 418 passed
- `make query-access-corpus-gates`: PASS
- `go test ./... -count=1`: 2972 passed
- `go test -tags postgresql ./... -count=1`: 4768 passed
- `golangci-lint run ./...`: clean
- `make decision-record-gate`: PASS
- `make release-gofmt-gate`: PASS
- Docker E2E: PG17 compose-backed integration had not yet been executed at the time of the prior acceptance claim. Status returned to `Proposed` pending real E2E evidence.

### T9 — Operator operand provenance (2026-07-13)

**Status remains:** `Proposed`. **Blocked with evidence** — oracle security
review identified 3 P1 concerns that must be resolved before shipping binary
`column OP column` promotion.

**What T9 delivers (implementation complete, blocked on security review):**

1. **Parser scope resolution enhanced:**
   - `buildSelectScope` recursively collects JOIN RangeVars with full provenance
   - Tracks CTE names to reject CTE-sourced columns
   - Tracks derived tables (RangeSubselect) to reject derived-table columns
   - `resolveColumnRef` returns Schema, Kind, and Resolved fields
   - Only resolves to base_table bindings (fail-closed for CTE/derived)

2. **Operand column provenance:**
   - Added `OperandColumnRef` type (Schema, Table, Column) to parser and application
   - `recordOperatorCandidate` collects provenance for column operands
   - Only records provenance for base_table columns (fail-closed for CTE/derived)

3. **Session-consistent type OID resolution:**
   - Added `ResolveColumnTypeOIDs` method to `EffectIdentityAdapter`
   - Queries `pg_attribute.atttypid` on the pinned session
   - `resolveAndProveEffects` calls `ResolveColumnTypeOIDs` via type assertion
   - Added `ColumnTypeOIDResolver` narrow interface

4. **PG17 Docker E2E:**
   - `TestTrustedService_PG17JoinComparisonE2E` proves JOIN promotion through
     real Service.Analyze with pinned session, EffectIdentityAdapter, manifest,
     and real table structure
   - Analyzes `SELECT u.name, o.user_id FROM app.users u JOIN app.orders o ON u.id = o.user_id`
   - Asserts: `read_only`, `admissible`, no `unproven_operator_effect`

5. **Negative test cases (11 scenarios):**
   - literal, param, NULL, cast, type mismatch, CTE, derived, view, ambiguous,
     custom operator, non-manifest operator — all remain indeterminate

**P1 concerns (blocking — must fix before promotion):**

1. **Catalog snapshot not consistent.** `service.go:195` calls `ResolveColumnTypeOIDs`
   on the pinned connection, then the adapter calls `ResolveEffectIdentities`; these
   are not wrapped in a single `REPEATABLE READ READ ONLY` transaction. Concurrent
   DDL can cause type facts and identity facts to come from different catalog
   snapshots, allowing incorrect promotion. **Fix required:** combine into one
   atomic proof operation under `REPEATABLE READ`. **FIXED:** Added
   `ResolveColumnTypesAndEffectIdentities` method using `REPEATABLE READ READ ONLY`
   transaction via `txCatalog` wrapper. `service.go` now requires `AtomicProofResolver`
   and rejects promotion if the resolver doesn't implement it. Non-atomic fallback
   path removed entirely.

2. **Unqualified relation bound to DefaultSchema.** `query_access.go:903` assigns
   `defaultSchema` to every unqualified RangeVar. This is not PostgreSQL `search_path`
   resolution; same-name relation shadowing can yield wrong column types and
   incorrect admission. **Fix required:** resolve each relation on the pinned
   session to its canonical relation OID and namespace via `search_path`, then
   resolve `pg_attribute` by `attrelid`. **FIXED:** `addRangeVarToScope` now sets
   empty Schema for unqualified relations. `ResolveColumnTypeOIDs` uses
   `resolveColumnTypeOIDBySearchPath` to walk the session's search_path.
   `relationFromRangeVar` also leaves Schema empty for unqualified relations to
   ensure consistency between requirements and type resolution. `resolveAndProveEffects`
   now rejects promotion entirely when unqualified relations exist with operator
   candidates that have column operands, because we cannot verify that `defaultSchema`
   matches the pinned session's first search_path entry.

3. **CTE scope not inherited into nested SELECTs.** `query_access.go:843` creates
   a new CTE map for each `buildSelectScope` call; nested SELECTs cannot see outer
   CTEs. Must ensure CTE/derived never generate operand provenance, either by
   fixing lexical scope inheritance or by making such queries fully fail-closed.
   **FIXED:** `buildSelectScope` now accepts `parentCTENames` parameter. CTE names
   are propagated through `collectSelectEffects` → `collectNodeEffects` → nested
   `buildSelectScope` calls. `resolveColumnRef` checks `cteNames` and returns
   `Kind: "cte", Resolved: false` for CTE-sourced columns. Sibling CTEs in the
   same WITH clause now see each other via accumulated `cteNames` map. Recursive
   CTE self-references now see their own name before body processing.

**P2 concerns (should-fix):**

4. **Three-part unbound column ref fails open.** `query_access.go:622` returns
   `Kind: "base_table", Resolved: true` when no binding matches a three-part
   reference. Must return unresolved — unproven schema.table.column is not
   physical provenance. **FIXED:** Both two-part and three-part unbound references
   now return `Kind: "unknown", Resolved: false`.

5. **Internal result lacks JSON blocking.** `contracts.go:93` `EffectCandidates`
   should have `json:"-"`. Even though current transports do not leak it, callers
   should not be able to marshal internal fields by accident. **FIXED:** Added
   `json:"-"` tag to `EffectCandidates` field.

6. **GitNexus pre-commit discipline not completed.** Index was stale after commit
   but `detect_changes` was not run. Must refresh index and run impact/detect per
   repo rules before committing. **FIXED:** Ran `npx gitnexus analyze` to refresh
   index, then ran `gitnexus_detect_changes` to verify scope.

**Artifacts:**

- `internal/infrastructure/parser/postgresql/query_access.go` (scope resolution)
- `internal/infrastructure/parser/postgresql/query_access_effect_candidates.go` (OperandColumnRef)
- `internal/infrastructure/parser/postgresql/query_access_scope_provenance_postgresql_tag_test.go` (new)
- `internal/application/queryaccess/contracts.go` (OperandColumnRef)
- `internal/application/queryaccess/extract_postgresql.go` (provenance mapping)
- `internal/application/queryaccess/identity_resolver.go` (ColumnTypeOIDResolver)
- `internal/application/queryaccess/service.go` (ResolveColumnTypeOIDs via type assertion)
- `internal/infrastructure/metadata/postgresql/effect_identity_resolver.go` (ResolveColumnTypeOIDs)
- `internal/infrastructure/metadata/postgresql/effect_identity_session.go` (columnTypeOID)
- `internal/infrastructure/metadata/postgresql/effect_identity_resolver_test.go` (unit tests)
- `internal/infrastructure/metadata/postgresql/effect_identity_resolver_integration_test.go` (E2E)
- `internal/application/queryaccess/trusted_service_postgresql_tag_test.go` (negative tests)
- `internal/application/queryaccess/extract_postgresql_tag_test.go` (provenance tests)
- `testdata/query-access/postgresql/select_with_join_on.expected.yaml` (updated)

**Verification evidence:**

- `go test ./...` — 2972 passed
- `go test -tags postgresql ./...` — 4763 passed
- `go test -tags postgresql,integration` — E2E JOIN promotion passes
- `go test -race` — 367 passed (no race conditions)
- `golangci-lint run ./...` — 1 false-positive ineffassign (intentional pattern)
- `make query-access-corpus-gates` — PASS
- `make decision-record-gate` — PASS
- `make release-gofmt-gate` — PASS

**Kill criteria assessment:** Oracle P1 concerns relate to search_path shadowing
and catalog snapshot consistency — not to the kill criteria in §9 (name allowlists,
volatility allowlists, or unbounded manifests). The manifest identity proof model
remains sound; the issues are in the operand provenance binding layer.

### T11 — Security Hardening (2026-07-14)

**Status remains:** `Proposed`. **Implemented** — fixes three security gaps found
during independent Oracle/Momus review.

**What T11 delivers:**

1. **SQLValueFunction treated as unproven effect (S4):** PostgreSQL
   `SQLValueFunction` expressions (`current_user`, `session_user`,
   `current_database`, `current_role`, `current_schema`, `current_date`,
   `current_timestamp`, `localtime`, `localtimestamp`, `user`) are now recorded
   as synthetic function candidates and emit `unproven_function_effect` reason
   codes. Previously they were silently ignored like literals.

2. **EXPLAIN relation extraction fix (Oracle P1):** `ExtractQueryAccess` now
   extracts relations/columns/outputs from the inner query of regular `EXPLAIN`
   statements. `EXPLAIN ANALYZE` (which executes the query) still returns no
   relations (classified as `not_read_only`). This prevents EXPLAIN from bypassing
   the unqualified-relation barrier.

3. **Default service unqualified barrier (S1):** The unqualified-relation barrier
   now runs when there's a `SchemaResolver` (not just when there's a trusted
   bundle). This prevents `NewService()` with a resolver from resolving unqualified
   relations to `DefaultSchema` and producing physical requirements.

**Oracle findings addressed:**

| Finding | Severity | Fix |
|---------|----------|-----|
| No-trusted PG can become admissible with resolver | Critical | Barrier now runs when SchemaResolver present |
| EXPLAIN bypasses relation extraction | Critical | Extract inner query relations for regular EXPLAIN |
| SQLValueFunctions silently treated like literals | High | Record as unproven function effects |
| Search-path drift accepted for unqualified | Medium | Already handled by GateIdentityBatchAgainstLiveContext |
| Relation metadata not bound to same session | Medium | Documented as limitation (deferred) |

**Momus findings addressed:**

| Finding | Fix |
|---------|-----|
| T11.1-T11.2 barrier only for trusted | Fixed: barrier runs when SchemaResolver present |
| S7 gate location clarification | Documented: gate is inside adapter, not application |
| T11.6 invalid window syntax | Fixed: replaced with valid PARTITION BY syntax |

**New test coverage:**

- `TestExtractQueryAccess_UnprovenEffectReasonCodes/sqlvalue_*` (13 cases)
- `TestDefaultService_UnqualifiedRelation_FailClosed` (S1 proof)
- `TestAnalyzeQueryAccess_UnqualifiedPostgreSQL_FailClosed` (SDK S1 proof)
- `TestEffectIdentity_StructuralBoolExpr_NotCatalogIdentityProof` (updated for barrier)

**Verification evidence:**

- `go test ./internal/domain/queryaccess/...` — 87 passed
- `go test ./internal/application/queryaccess/...` — 213 passed
- `go test -tags postgresql ./internal/application/queryaccess/...` — 425 passed
- `go test ./internal/infrastructure/parser/postgresql/...` — 1 passed
- `go test -tags postgresql ./internal/infrastructure/parser/postgresql/...` — 623 passed
- `go test ./pkg/deltascope/...` — 67 passed
- `go test -tags postgresql ./pkg/deltascope/...` — 373 passed
- `gofmt` — clean
- `go build ./...` — exit 0
- `go build -tags postgresql ./...` — exit 0
- `go vet ./...` — exit 0
- `go vet -tags postgresql ./...` — exit 0
- GitNexus detect_changes — 8 files changed, all within expected scope

**Public delivery boundary (unchanged):**

- No public trusted SDK/CLI/HTTP promotion path added
- Default SDK, CLI, and HTTP remain fail-closed
- Future public promotion API requires: explicit caller-owned pinned-session,
  execution-ownership contract, separate design review, separate approval
- MCP remains without query-access tool

### T12 — Adversarial Tests and Documentation (2026-07-14)

**Status remains:** `Proposed`. **Implemented** — adds adversarial tests and documentation only. Does NOT modify production code. Production safety defects remain for T13.

**What T12 delivers:**

1. **Adversarial test coverage:**
   - `TestAdversarial_WrongSessionBinding` → indeterminate
   - `TestAdversarial_WrongDatabaseOID` → indeterminate
   - `TestAdversarial_WrongRoleOID` → indeterminate
   - `TestAdversarial_WrongServerVersion` → indeterminate
   - `TestAdversarial_WrongPathEpoch` → indeterminate
   - `TestAdversarial_WrongSearchPath` → indeterminate
   - `TestAdversarial_FactDatabaseMismatch` → indeterminate
   - `TestAdversarial_FactVersionMismatch` → indeterminate
   - `TestAdversarial_IncompleteBatch` → indeterminate
   - `TestAdversarial_NoLeak_PublicJSON` → no OIDs/session/SQL/credentials/severity
   - `TestAdversarial_NoLeak_ReasonCodes` → no catalog names/OIDs/SQL
   - `TestAdversarial_NoLeak_Unresolved` → no catalog names/OIDs

2. **Documentation updates:**
   - Decision record evidence sections
   - Test characterization documentation

**Explicit non-claims of T12:**

- T12 does NOT modify production code
- T12 does NOT fix the conditional unqualified-relation barrier
- T12 does NOT fix the atomic resolver contract
- T12 does NOT fix PostgreSQL traversal gaps
- T12 does NOT fix schema provenance issues
- Production safety defects identified in T13 plan remain

**File scope (tests + docs only):**

- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- `internal/application/queryaccess/adversarial_postgresql_tag_test.go`
- `internal/application/queryaccess/service_test.go`
- `internal/application/queryaccess/trusted_service_postgresql_tag_test.go`

**Verification evidence:**

- `git diff --name-only 24c606b..a7c3cbb` confirms only docs and test files changed
- All adversarial tests pass
- No production code modified

### T13 — Production Safety Remediation (2026-07-14)

**Status remains:** `Proposed`. **Implemented** — implements consolidated proof-gateway with invariant-driven validation.

**What T13 delivers:**

1. **Consolidated proof-gateway function:**
   - `resolveAndProveEffects` is the sole promotion path for PostgreSQL
   - All context validation in one place via `ValidateResolutionContextForPromotion`
   - All fact pinning validation in one place via `ValidateFactPinning`
   - All ordinal validation in one place via `ValidateBatchOrdinals`
   - Gateway flow enforces INV-1 through INV-14 invariants

2. **Invariant enforcement:**
   - INV-1: Single proof gateway (`resolveAndProveEffects`)
   - INV-2: Gateway receives initial/final context, candidates, identity batch
   - INV-3: Context compatibility fail-closed (`ValidateResolutionContextForPromotion`)
   - INV-4: Database OID pin (`ValidateFactPinning`)
   - INV-5: Server version pin (`ValidateFactPinning`)
   - INV-6: Ordinal fail-closed (`ValidateBatchOrdinals`)
   - INV-7: Search-path drift fail-closed (`ValidateResolutionContextForPromotion`)
   - INV-8: Unqualified relation barrier (unconditional)
   - INV-9: Unknown AST fail-closed (parser emits unproven_* reasons)
   - INV-10: Static analysis limitation (documented)
   - INV-11: TrustPolicy gate (IsTrusted called only after validation)
   - INV-12: Defense-in-depth against contract-violating adapter output (fact pinning + ordinal validation + operand-type binding for binary operators)
   - INV-13: No public trusted API (NewTrustedService internal-only)
   - INV-14: No leaks (json:"-" on EffectCandidates, bounded reason codes)

3. **Strengthened AtomicProofResolver contract:**
   - Documented INV-12 defense-in-depth against contract-violating adapter output
   - Facts must be stamped with DatabaseOID and ServerVersionNum
   - Application validates fact pinning before IsTrusted
   - Application validates batch ordinals before completion
   - Application validates operand-type binding (type-map cross-check for binary operators; nil/empty/missing/unexpected map fails closed)
   - Removed dead cast-name fields (CastSourceTypeName, CastTargetTypeName): Phase 1 does not trust casts

4. **Complete parser fail-closed traversal:**
   - Set-operation branches, CTE bodies, nested/scalar subqueries handled
   - JOIN USING emits unsupported_traversal reason
   - Three-part references preserve schema
   - Output lineage correctly parsed

5. **Schema provenance preservation:**
   - Schema flows end-to-end from parser to domain
   - Qualified references honor explicit schema
   - Unqualified references remain empty schema

6. **SQL value functions and EXPLAIN fail-closed:**
   - SQLValueFunction emits unproven_function_effect
   - EXPLAIN ANALYZE is not_read_only
   - Regular EXPLAIN extracts inner query relations

7. **Comprehensive adversarial test coverage:**
   - 16 adversarial tests covering INV-3 through INV-12
   - Wrong session binding, database OID, role OID, server version, path epoch, search path
   - Fact database mismatch, version mismatch
   - Wrong candidate ordinal, duplicate ordinal, foreign fact tuple, wrong canonical signature
   - Incomplete batch, no-leak assertions

**Verification evidence:**

- `go test ./internal/application/queryaccess/... -count=1`: 213 passed
- `go test -tags postgresql ./internal/application/queryaccess/... -count=1`: 437 passed
- `go test -tags postgresql ./internal/application/queryaccess/... -run TestAdversarial -count=1`: 16 passed
- `go test -tags postgresql ./internal/infrastructure/parser/postgresql/... -count=1`: 629 passed
- `make query-access-corpus-gates`: PASS
- `gofmt -w` on modified Go files: clean
- `go build ./...`: exit 0
- `go build -tags postgresql ./...`: exit 0
- `go vet ./...`: exit 0
- `go vet -tags postgresql ./...`: exit 0

**Static-analysis limitation:** Static analysis produces requirements only. It is not grant evaluation, authorization, or a guarantee that a later execution uses the same database snapshot.

### T14 — Strict-Requirements Completeness and TABLESAMPLE Fail-Close (2026-07-15)

**Status remains:** `Proposed`.

**What T14 delivers:**

1. **Extended column reference collection:**
   - `collectColumnReferences` now walks DISTINCT ON, window partition/order/frame, aggregate FILTER, and LIMIT/OFFSET expressions
   - New `collectWindowRefs` helper walks WindowDef partition, order, and frame bounds
   - New domain usage contexts: `UsageDistinctOn = "distinct_on"`, `UsageLimit = "limit"`
   - `UsageFilter` comment clarified to cover both WHERE and aggregate FILTER

2. **TABLESAMPLE fail-close:**
   - `collectNodeEffects` sets `unsupportedTraversal` flag immediately for `Node_RangeTableSample` without walking children
   - `walkFromClause` unwraps TABLESAMPLE underlying relation for relation reporting
   - Reuses existing `unsupported_traversal` reason code (no new public contract surface)

3. **Candidate-to-fact binding validation:**
   - `ValidateCandidateFactBinding` validates resolved facts match expected candidate shape
   - Checks: kind match, nonzero ObjectOID, operator arity 1-2, function arity match, cast arity 1
   - On mismatch: converts to `lookup_failed` and removes facts before manifest proof
   - Wired in `resolveAndProveEffects` after fact pinning validation

4. **Corpus fixtures:**
   - Revised: `select_agg_filter_function` (now includes `users.name` with `filter` usage)
   - Revised: `select_window_partition_function` (now includes `users.id` with `ordering` and `users.name` with `window`)
   - New: `select_distinct_on_column` (DISTINCT ON column reference)
   - New: `select_limit_subquery` (LIMIT subquery column reference)
   - New: `select_tablesample` (TABLESAMPLE fail-close)

**Public contract changes:**

- New usage context values `distinct_on` and `limit` in `referenced_columns[].usages`
- `UsageFilter` documentation clarified (behavior unchanged)
- Consumers must tolerate unknown usage strings

**Verification evidence:**

- `go test ./internal/domain/queryaccess/... -count=1`: 87 passed
- `go test ./internal/application/queryaccess/... -count=1`: 213 passed
- `go test -tags postgresql ./internal/application/queryaccess/... -count=1`: 444 passed
- `go test ./internal/infrastructure/parser/postgresql/... -count=1`: 1 passed
- `go test -tags postgresql ./internal/infrastructure/parser/postgresql/... -count=1`: 629 passed
- `make query-access-corpus-gates`: PASS (38 corpus cases)
- `make pg-unit-test-gates`: PASS
- `golangci-lint run ./...`: clean
- `go vet ./...` / `go vet -tags postgresql ./...`: clean
- `go test -race ./...`: clean
- `npm test --prefix packages/deltascope-mcp`: 15 passed
- GitNexus detect_changes: 10 changed symbols, 5 affected processes, medium risk

## Consequences

- Implementation must research and publish a version-scoped effect identity
  manifest **before** promoting any production admission paths.
- Identity resolver returns facts; application/domain trust policy applies the
  manifest. Callers must not supply a self-claimed `Trusted` bit.
- Implementation must thread type facts from relation metadata into effect
  resolution.
- Callers who want PostgreSQL `admissible` must supply a catalog-capable
  resolver (or an equivalent frozen snapshot) with a resolution context locked
  to execution.
- Documentation (reference + recipe) must replace "PostgreSQL admission is
  always indeterminate" with "unproven or non-manifest effects remain
  indeterminate; manifest-listed proven builtins may be admissible".
- Future work that adds new manifest entries, volatile builtins, or
  non-`pg_catalog` trust requires a new decision record (or an explicit
  amendment to this one with re-audit evidence).

## Kill criteria (do not implement product code)

If research during implementation shows any of the following, stop and keep an
audit spike only:

1. Operand/result types cannot be inferred with a **bounded** algorithm for the
   first support matrix without full planner reimplementation.
2. Catalog identity cannot be uniquely resolved without accepting
   `search_path` shadowing of builtins.
3. Required catalog access cannot be done without leaking connection secrets
   into public results or requiring unsafe privileges.
4. The only workable approach reduces to **name allowlists** without OID
   binding.
5. The only workable approach reduces to a **generic volatility allowlist**
   (`pg_catalog` + `i|s`) without a per-item audited, version-scoped manifest.
6. A **bounded, version-scoped, maintainable** trusted-effect manifest cannot
   be defined for the phase-1 matrix (or maintenance cost forces open-ended
   class trust).

## Links

- Design: `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md`
- Implementation plan: `docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md`
- OMO prompts: `docs/plans/2026-07-12-query-access-pure-read-admissibility-omo-prompts.md`
- Prior foundation: `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
- CTE/lineage: `docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`
