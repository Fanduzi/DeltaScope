# Decision: Query Access Pure-Read Admissibility via Proven Identity

Date: 2026-07-12
Status: Proposed
Related milestone/version: (unassigned; branch `query-access-pure-read-admissibility`)
Related commits:
- (none yet; design-only Task 1; trust-policy tightening in progress)
Related tests:
- Planned: extended `testdata/query-access/` corpus (PostgreSQL positive + adversarial)
- Planned: unit/integration tests for effect-identity catalog resolution
- Planned: adversarial characterization that `pg_catalog` + volatility alone never promotes
Related docs:
- `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md`
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

(To be filled when implementation lands. Design-time characterization:)

- Current PG corpus: 22/22 `admission: indeterminate`.
- Current code: `pgContainsOperatorExpr` returns true on any `A_Expr`/`TypeCast`;
  `pgContainsFunctionCallInExpr` on any `FuncCall`; no OID capture.
- `reclassifyAfterResolution` hard-stops PostgreSQL lifts.
- Relation `SchemaResolver` queries `pg_class`/`pg_attribute` only; no
  `pg_operator` / `pg_proc` / `pg_cast`.
- Trust model: catalog facts + audited manifest (not volatility class).

Planned gates: corpus matrix (safe positives + adversarial negatives including
non-manifest `pg_catalog` stable, `current_setting`, `pg_get_*`, function-
backed cast), unit tests for identity resolver (facts only), trust policy
manifest tests, cross-surface SDK/CLI/HTTP parity, no-leak tests,
`make query-access-corpus-gates`, dialect-tagged PostgreSQL tests.

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
