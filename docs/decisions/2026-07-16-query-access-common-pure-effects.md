# Decision: Query Access Common Pure-Effect Admissibility

Date: 2026-07-16
Status: Proposed
Related milestone/version: Unassigned
Related commits:
- Planning baseline: `v0.390.0` (`d72eeb4`)
Related tests:
- Current TiDB AST characterization and Query Access corpus
- PostgreSQL trusted-SDK PG17 integration suite
Related docs:
- `docs/plans/2026-07-16-query-access-common-pure-effects-spec.md`
- `docs/plans/2026-07-16-query-access-common-pure-effects-design.md`
- `docs/plans/2026-07-16-query-access-common-pure-effects-implementation.md`
- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`

## Context

Query Access recognizes table and column dependencies for many read-only
shapes, including grouping and window clauses. However, the shared
MySQL/TiDB parser records every function, aggregate, and window function as a
`function_call`; the empty V1 function allowlist then makes the result
`indeterminate`. Common reporting queries such as `COUNT(*)`, `SUM(amount)`,
and `ROW_NUMBER() OVER (...)` therefore cannot become admissible even when
their physical requirements are complete.

PostgreSQL has a narrower, stronger path. v0.390.0 exposes a caller-owned,
PG17, catalog-bound trusted SDK path. Its manifest proves only a small set,
including `count` and selected comparison operators. More common aggregates
and window functions remain indeterminate until their actual catalog identity
and requirements are proven.

This is a product capability gap, not a reason to weaken fail-closed behavior.
`admissible` means DeltaScope derived complete, known static requirements for a
demonstrably read-only query. It does not mean a principal has permissions,
that a function is harmless because of its spelling, or that a later execution
uses the analyzed database snapshot.

## Decision

DeltaScope will pursue a bounded Common Pure-Effect Admissibility milestone.
It may admit a function-bearing query only after all of these gates pass:

1. The parser has traversed every executable expression position relevant to
   the statement and extracted complete strict-mode dependencies.
2. Every referenced relation and column is uniquely resolved to a permitted
   physical source; wildcard, view, derived/CTE, ambiguity, and missing
   metadata boundaries remain fail-closed unless separately proven.
3. Every effect candidate is in the dialect's bounded Phase 1 set and is
   independently proven by that dialect's approved proof mechanism.
4. No unsupported expression, unproven function/operator/cast, resolver
   failure, coercion gap, or incomplete proof remains.

The Phase 1 target is deliberately small:

| Class | Candidate forms after proof |
| --- | --- |
| Aggregates | `COUNT(*)`, `COUNT(base_column)`, `SUM(base_column)`, `AVG(base_column)`, `MIN(base_column)`, `MAX(base_column)` |
| Windows | `ROW_NUMBER()`, `RANK()`, `DENSE_RANK()` with direct-column `OVER` partition/order definitions |

Phase 1 excludes `DISTINCT`, aggregate `FILTER`, aggregate-local ordering,
ordered-set aggregates, nested scalar expressions, window frames, named
windows, recursive/derived/CTE effect inputs, views, wildcard inputs,
parameters, and casts. An excluded construct remains `indeterminate`; this
list expands only through a new proof and corpus decision.

### PostgreSQL proof

PostgreSQL promotion continues to require the existing caller-owned session
path. The actual `pg_proc`/aggregate/window identity, argument types, version,
database, role, and session context must match an audited PG17 manifest entry.
A function name, `pg_catalog` label, volatility class, or parser shape is
never sufficient. The default SDK, CLI, and HTTP paths remain fail-closed and
do not acquire database connections.

### MySQL and TiDB proof feasibility

MySQL and TiDB must not inherit PostgreSQL's OID policy or receive a syntax
name allowlist by assumption. Before either dialect promotes a builtin, the
milestone must establish a separately audited, version-scoped identity model
covering stored functions, UDFs/plugins, qualification, SQL mode, and server
version. A model may be static only if that conclusion is defensible for the
supported version range; otherwise it must be session/catalog-bound.

If a dialect cannot provide bounded identity proof, it keeps the current
`unknown_function_effect` behavior. The milestone may ship a proven subset for
another dialect, but must state that distinction in the public contract.

## Rationale

Common aggregates and ranking windows are high-value reporting constructs, but
their function nodes are not a sufficient semantic proof. Keeping all of them
indeterminate forever makes Query Access less useful; treating every matching
name as a builtin would create a false-admission boundary. A bounded candidate
set, complete strict requirements, and a dialect-specific identity model give
the product a path between those two failures.

PostgreSQL already demonstrates the required shape: catalog facts bound to a
caller-owned session are checked against an application-owned manifest. MySQL
and TiDB need an independent feasibility decision because their function
resolution and metadata model must not be inferred from PostgreSQL or from a
shared parser implementation.

## Public Contract

- A proven Phase 1 query may return `read_classification: read_only` and
  `admission: admissible` only when all ordinary metadata and proof gates pass.
- In `strict` mode, requirements include aggregate arguments, grouping,
  having, window partition/order, and every other expression dependency that
  influences rows or outputs. `projection_only` retains its inference-risk
  warning and never becomes equivalent to strict.
- Existing bounded reason codes remain machine identifiers only. No result,
  error, CLI output, or HTTP JSON may expose function names, catalog facts,
  SQL/literals, credentials, DSNs, session data, or `severity`.
- PostgreSQL promotion remains opt-in through
  `AnalyzePostgreSQLQueryAccessWithSession`; it is not authorization and does
  not guarantee a later execution snapshot.
- MySQL/TiDB default behavior does not become more permissive until their
  proof feasibility gate passes and the resulting surface contract is tested.

## Deferred / Out Of Scope

- Arbitrary scalar functions, UDFs, stored functions, plugins, dynamic SQL,
  file/network/session-mutating functions, and user-defined operators/casts.
- `DISTINCT`, `FILTER`, ordered-set aggregates, aggregate-local ordering,
  arbitrary expression arguments, named windows, frames, and windowed
  aggregates in Phase 1.
- View expansion, RLS, masking, grant evaluation, authorization, SQL rewrite,
  and execution-affinity enforcement.
- CLI/HTTP live database connections and an MCP Query Access tool.
- A claim of full MySQL, TiDB, or PostgreSQL function grammar coverage.

## Verification Evidence Required Before Acceptance

- Characterization tests prove current rejection and each candidate's
  strict/projection-only dependency set across relevant AST positions.
- A dialect/version research ledger records trust root, shadowing behavior,
  negative cases, and a kill decision for PostgreSQL, MySQL, and TiDB.
- PostgreSQL PG17 Docker E2E calls the public trusted SDK and proves an effect
  only where exact manifest identity succeeds.
- MySQL/TiDB E2E or version-scoped proof tests prove any newly admissible
  builtin; no name-only promotion is accepted.
- Corpus fixtures cover positive, negative, metadata-failure, ambiguity,
  wildcard, no-leak, and cross-surface cases.
- An independent final audit verifies no default PostgreSQL/CLI/HTTP/MCP
  promotion leak and no regression to existing MySQL/TiDB admissible SELECTs.

## Verification Evidence

At proposal time, evidence is limited to the existing V1 boundary:

- TiDB AST characterization proves aggregate/window nodes currently trigger
  the empty-function-allowlist `indeterminate` path.
- Existing parser extraction already records grouping, having, and window
  partition/order dependencies, but Phase 1 completeness has not yet been
  independently established for every candidate and expression holder.
- v0.390.0 PostgreSQL trusted SDK integration proves only its current PG17
  manifest subset. It is not evidence for new aggregates or windows.

No dialect has new promotion evidence under this proposal. The required proof,
corpus, cross-surface, and audit evidence above is the acceptance threshold.

## Consequences

The milestone favors a small useful set of reporting queries over a broad but
unverifiable function allowlist. It may stop one dialect without blocking a
separately proven dialect. This decision remains `Proposed` until proof,
corpus, public-surface, and audit evidence exists.

## Links

- Current PostgreSQL pure-read decision:
  `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Current Query Access reference:
  `docs/reference/query-access-analysis.md`
- Specification:
  `docs/plans/2026-07-16-query-access-common-pure-effects-spec.md`
- Design:
  `docs/plans/2026-07-16-query-access-common-pure-effects-design.md`
- Implementation plan:
  `docs/plans/2026-07-16-query-access-common-pure-effects-implementation.md`
