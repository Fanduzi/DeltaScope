# Specification: Query Access Common Pure-Effect Admissibility

Date: 2026-07-16
Status: Proposed
Decision: `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
Baseline: v0.390.0

## Product Requirement

A query platform needs to accept ordinary reporting SQL when data sources and
effect semantics are fully known. Today `COUNT`, `SUM`, and basic window
functions frequently receive `indeterminate` solely because a parser detected
a function node. The platform cannot then use structured requirements for its
own authorization decision.

The outcome is not "functions are safe". The outcome is a bounded set of
functions for which DeltaScope can prove complete static requirements and
return `admissible` rather than forcing a blanket rejection.

## Terms

- **Pure effect**: a bounded builtin whose evaluation reads only row sources
  already represented in Query Access requirements and has no hidden relation,
  file, network, role, configuration, mutation, or execution-control effect.
- **Identity proof**: dialect-specific evidence that the executed construct is
  the audited builtin, not a name collision, overload, UDF, cast, or plugin.
- **Dependency completeness**: every physical table/column influencing the
  result is represented in strict-mode requirements before admission can lift.
- **Proof provider**: an internal dialect-specific component that produces
  facts for a policy; it never returns a public `Trusted` flag.

## Phase 1 Candidate Matrix

This matrix is an implementation target, not an automatic allowlist. A row is
admissible only after its dialect proof gate succeeds.

| SQL shape | Strict requirements that must exist | Phase 1 result when proven |
| --- | --- | --- |
| `COUNT(*)` over one physical base relation | `read_table` | admissible |
| `COUNT(col)` | table + `col` projection source | admissible |
| `SUM/AVG/MIN/MAX(col)` | table + `col` projection source | admissible |
| Aggregate with `GROUP BY group_col` | aggregate input + `group_col` grouping source | admissible |
| `ROW_NUMBER() OVER (PARTITION BY p ORDER BY o)` | table + `p` window source + `o` window/ordering source | admissible |
| `RANK` / `DENSE_RANK` with direct partition/order | same as row number | admissible |

For PostgreSQL the positive rows require the public trusted-session SDK and a
supported PG17 manifest. For MySQL/TiDB they require the dialect-specific proof
feasibility gate. A normal `AnalyzeQueryAccess` call must not become more
permissive merely because this specification exists.

## Mandatory Negative Matrix

All dialects remain `indeterminate` for these examples unless a later decision
explicitly promotes a narrower subcase:

| Shape | Reason |
| --- | --- |
| Unknown/UDF/stored/plugin function | identity and hidden-effect proof absent |
| Function with arbitrary/nested scalar expression argument | dependency/effect scope not Phase 1 |
| `DISTINCT`, aggregate `FILTER`, aggregate-local `ORDER BY` | separate dependency and semantic proof required |
| Named window, explicit frame, windowed aggregate | separate window semantics proof required |
| Cast/coercion, parameter, NULL, literal-dependent overload | type/coercion proof incomplete |
| Unqualified relation, view, CTE/derived effect input, wildcard | physical source or permission semantics incomplete |
| Ambiguous/missing metadata, resolver error, unsupported AST | fail-closed invariant |

Write operations, locking reads, `SELECT INTO`, external-file forms, and
multi-statements keep existing rejected/not-read-only behavior.

## Dialect Rules

### PostgreSQL

- Target PostgreSQL 17 only in Phase 1.
- Use the caller-owned `*sql.Conn` trusted SDK path. Default SDK, CLI, HTTP,
  and MCP do not promote function-bearing queries.
- Resolve exact `pg_proc`/aggregate/window identity and argument types in the
  pinned session. Match a version-scoped manifest entry and retain all context,
  ordinal, candidate/fact, and type binding checks.
- Do not trust `pg_catalog`, volatility, or spelling alone.

### MySQL and TiDB

- Treat MySQL and TiDB as separate proof domains even though they currently
  share the TiDB parser path.
- The research task must decide with version-scoped evidence whether builtin
  identity can be established without a live connection. It must examine
  stored-function/UDF/plugin shadowing, qualification, SQL mode, and version
  behavior.
- A parser-normalized builtin token is candidate input only, never proof.
- If a dialect has no adequate identity model, leave its function-bearing
  queries on `unknown_function_effect`; do not block a proven dialect.

## Result Invariants

- Proof candidates and facts are internal-only and use `json:"-"` or no public
  mapping.
- A failed proof never downgrades a rejection and never upgrades an
  indeterminate result.
- A successful proof removes only the reason belonging to its proven candidate;
  it cannot clear unrelated unresolved, unproven, or unsupported reasons.
- Strict requirements must be complete before `admissible`; projection-only
  keeps its existing warning contract.
- Reason codes remain bounded and deterministic. They never contain function
  names, OIDs, signatures, source names, SQL, literals, credentials, or driver
  errors.

## Exit Criteria

The milestone cannot be Accepted until proof, corpus, cross-surface, no-leak,
and independent-audit gates all pass. If MySQL or TiDB proof feasibility fails,
the specification is still satisfied only by preserving that dialect's
fail-closed behavior and documenting the deferred boundary.
