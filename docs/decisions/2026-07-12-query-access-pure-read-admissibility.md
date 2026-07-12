# Decision: Query Access Pure-Read Admissibility via Proven Identity

Date: 2026-07-12
Status: Proposed
Related milestone/version: (unassigned; branch `query-access-pure-read-admissibility`)
Related commits:
- (none yet; design-only Task 1)
Related tests:
- Planned: extended `testdata/query-access/` corpus (PostgreSQL positive + adversarial)
- Planned: unit/integration tests for effect-identity catalog resolution
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

## Decision

### 1. Public admission distribution may change under proven identity

When (and only when) every effect-bearing construct in the accepted read-only
subset has a **catalog-proven trusted identity**, PostgreSQL analysis may
return:

- `read_classification: read_only`
- `admission: admissible` (subject to existing unresolved/wildcard/ambiguity
  rules and mode requirements)

Unknown, unproven, ambiguous, or metadata-failed effects remain
`indeterminate`. Write/lock/file/session mutation paths remain `not_read_only`
→ `rejected` as today.

This is a **public behavior change** relative to v0.380.0's de facto
"PostgreSQL never admissible" distribution. Callers that treated every
PostgreSQL `indeterminate` as permanent will see some queries become
`admissible`. Callers that already fail-closed on `indeterminate` remain safe.

### 2. Identity proof, not syntax allowlists

Promotion from `indeterminate` → `read_only` for effect-bearing constructs
requires a **Catalog / Effect Identity Resolver** (extension of today's
relation-only `SchemaResolver`) that returns, at minimum:

| Construct | Must prove |
|-----------|------------|
| Operator (`A_Expr`) | Resolved operator OID; owning namespace is trusted (`pg_catalog`); operand types used in resolution; underlying function OID (`oprcode`) is trusted and has allowed volatility class |
| Function / aggregate (`FuncCall`) | Resolved function OID; owning namespace trusted; argument type list used in resolution; volatility class allowed; aggregate vs plain function distinguished |
| Cast (`TypeCast`) | Resolved cast path (source type OID → target type OID); implementation function (if any) trusted; no user-defined cast hijack |

**Not sufficient alone:**

- operator spelling `=`
- function name `count` / `COUNT`
- schema label strings without OID binding
- "looks like a builtin" heuristics

If type inference cannot produce a unique catalog match, or the match is
outside the trust set, or the resolver errors / returns unknown →
**do not promote**.

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
- **Wildcard not fully expanded / incomplete relation metadata:** indeterminate
  (unchanged).

### 4. Trust policy (minimum)

A resolved implementation is **trusted** only if all hold:

1. Owning namespace is `pg_catalog` (OID-bound, not name-guessed after path
   search alone).
2. Object is not a user-installable shadow accepted via caller `search_path`
   without explicit lock to a resolution context that matches execution.
3. Volatility is in the allowed set for pure-read admissibility:
   - **Immutable (`i`)** and **stable (`s`)** catalog functions are candidates.
   - **Volatile (`v`)** catalog functions stay indeterminate unless a later
     decision explicitly enumerates a frozen OID allowlist with rationale
     (default: no).
4. For operators: both the operator row and `oprcode` function must satisfy
   the trust policy.
5. User-defined operators/functions/casts in non-catalog schemas never trust.

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

## Rationale

- The foundation already defined the product scenario; the blocker is not MCP
  symmetry or a new flag, but **unproven effects**.
- PostgreSQL's overload and `search_path` model make name allowlists unsafe.
- `pg_query_go` parse trees expose operator/function **names and structure**,
  not OIDs; catalog proof is mandatory for promotion.
- Keeping fail-closed on missing proof preserves the foundation's security
  posture while unlocking only demonstrably safe shapes.

## Public Contract

After this decision is **Accepted** and implemented, consumers may rely on:

1. PostgreSQL queries that previously were always `indeterminate` may become
   `read_only` + `admissible` when every effect is catalog-proven trusted and
   all existing admission preconditions hold.
2. Absence of proof never yields `admissible`.
3. New bounded reason codes may appear for unproven effects; they are additive
   machine identifiers, not free-text SQL.
4. MySQL/TiDB admissible cases that exist today remain admissible unless a
   separate accepted decision says otherwise.
5. MCP tool list remains without query-access.
6. No `severity` field; no raw SQL / credential leakage in the public result.

Exact Go type names for the extended resolver remain implementation details
until the implementation plan locks them; the **proof requirements** above are
the contract principles.

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
- Changing audit rule evaluation or audit verdicts

## Verification Evidence

(To be filled when implementation lands. Design-time characterization:)

- Current PG corpus: 22/22 `admission: indeterminate`.
- Current code: `pgContainsOperatorExpr` returns true on any `A_Expr`/`TypeCast`;
  `pgContainsFunctionCallInExpr` on any `FuncCall`; no OID capture.
- `reclassifyAfterResolution` hard-stops PostgreSQL lifts.
- Relation `SchemaResolver` queries `pg_class`/`pg_attribute` only; no
  `pg_operator` / `pg_proc` / `pg_cast`.

Planned gates: corpus matrix (safe positives + adversarial negatives), unit
tests for identity resolver, cross-surface SDK/CLI/HTTP parity, no-leak tests,
`make query-access-corpus-gates`, dialect-tagged PostgreSQL tests.

## Consequences

- Implementation must thread type facts from relation metadata into effect
  resolution.
- Callers who want PostgreSQL `admissible` must supply a catalog-capable
  resolver (or an equivalent frozen snapshot) with a resolution context locked
  to execution.
- Documentation (reference + recipe) must replace "PostgreSQL admission is
  always indeterminate" with "unproven effects remain indeterminate; proven
  trusted builtins may be admissible".
- Future work that adds volatile builtins or non-`pg_catalog` trust requires a
  new decision record.

## Kill criteria (do not implement product code)

If research during implementation shows any of the following, stop and keep an
audit spike only:

1. Operand/result types cannot be inferred with a **bounded** algorithm for the
   first support matrix without full planner reimplementation.
2. Catalog identity cannot be uniquely resolved without accepting
   `search_path` shadowing of builtins.
3. Required catalog access cannot be done without leaking connection secrets
   into public results or requiring unsafe privileges.
4. The only workable approach reduces to name allowlists without OID binding.

## Links

- Design: `docs/plans/2026-07-12-query-access-pure-read-admissibility-design.md`
- Implementation plan: `docs/plans/2026-07-12-query-access-pure-read-admissibility-implementation.md`
- OMO prompts: `docs/plans/2026-07-12-query-access-pure-read-admissibility-omo-prompts.md`
- Prior foundation: `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
- CTE/lineage: `docs/decisions/2026-07-11-cte-derived-table-lineage-resolution.md`
