# Decision: Query Access Pure-Read Admissibility via Proven Identity

Date: 2026-07-12
Status: Proposed
Related milestone/version: (unassigned; branch `query-access-pure-read-admissibility`)
Related commits:
- Design + trust-policy docs on branch `query-access-pure-read-admissibility`
- T2 research commit: `docs: research query access effect identity manifest`
- T3 characterization commit: `test: characterize query access effect identity candidates`
- T4 reason codes commit: `feat: explain unproven query access effects`
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
- Planned (later): unit/integration tests for effect-identity catalog resolution + promotion
- Planned (later): positive corpus under fake/real identity resolver + manifest
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
