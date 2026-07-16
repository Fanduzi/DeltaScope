# Decision: Query Access Common Pure-Effect Admissibility

Date: 2026-07-16
Status: Proposed
Related milestone/version: Unassigned
Related commits:
- Planning baseline: `v0.390.0` (`d72eeb4`)
Related tests:
- Current TiDB AST characterization and Query Access corpus
- PostgreSQL trusted-SDK PG17 integration suite
- `internal/infrastructure/metadata/postgresql/pure_effect_feasibility_integration_test.go`
- `internal/infrastructure/metadata/mysql/pure_effect_feasibility_test.go`
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

For this research gate, PostgreSQL proceeds to the existing session-bound
catalog proof path. MySQL and TiDB remain deferred for Phase 1; no shared-parser
assumption promotes either dialect.

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

The feasibility gate separates proof-provider evidence from production
promotion. The tests below record the evidence without changing the manifest or
admission code.

The Phase-1 proof gateway now also validates candidate shape before identity
resolution. Only count-star, direct single-column function arguments, and
frame-free ranking windows may proceed; filters, distinct arguments, nested or
non-column operands, frames, and casts remain fail-closed. Ineligible
candidates force `has_unproven`, while `removeUnprovenEffectReasons` remains
reachable only after `all_proven`.

Coverage: `internal/application/queryaccess/pure_effect_proof_gateway_postgresql_tag_test.go`
and the existing candidate-binding adversarial suite verify eligible shapes,
ineligible shapes, partial batches, swapped facts, and candidate no-leak
behavior.

### PostgreSQL 17 — GO for the existing session-bound proof path

The PG17 integration test queries `pg_catalog.pg_proc` in the same Docker
environment used by the trusted SDK integration suite. The `pg_catalog`
namespace is OID 11 and all listed immutable rows have `provolatile = 'i'`.

Window identities:

| Candidate | OID | Signature | `prokind` | Arity | Volatility |
| --- | ---: | --- | --- | ---: | --- |
| `row_number` | 3100 | `pg_catalog.row_number()` | `w` | 0 | `i` |
| `rank` | 3101 | `pg_catalog.rank()` | `w` | 0 | `i` |
| `dense_rank` | 3102 | `pg_catalog.dense_rank()` | `w` | 0 | `i` |

Ordered-set negatives with the same spellings are distinct aggregate
identities and must not be accepted as window identities:

| Candidate | OID | Signature | `prokind` |
| --- | ---: | --- | --- |
| `rank` | 3986 | `VARIADIC "any" ORDER BY VARIADIC "any"` | `a` |
| `dense_rank` | 3992 | `VARIADIC "any" ORDER BY VARIADIC "any"` | `a` |

Existing count identities remain present:

| Candidate | OID | Argument OID | Result OID | `prokind` |
| --- | ---: | ---: | ---: | --- |
| `count(*)` | 2803 | — | 20 (`int8`) | `a` |
| `count(anyelement)` | 2147 | 2276 | 20 (`int8`) | `a` |

The common `app.users`/`app.orders` type rows are also present and immutable:

| Aggregate | OID | Argument OID | Result OID |
| --- | ---: | ---: | ---: |
| `sum(bigint)` | 2107 | 20 | 1700 (`numeric`) |
| `sum(integer)` | 2108 | 23 | 20 (`int8`) |
| `sum(numeric)` | 2114 | 1700 | 1700 (`numeric`) |
| `avg(bigint)` | 2100 | 20 | 1700 (`numeric`) |
| `avg(integer)` | 2101 | 23 | 1700 (`numeric`) |
| `avg(numeric)` | 2103 | 1700 | 1700 (`numeric`) |
| `min(bigint)` | 2131 | 20 | 20 (`int8`) |
| `min(text)` | 2145 | 25 | 25 (`text`) |
| `min(numeric)` | 2146 | 1700 | 1700 (`numeric`) |
| `max(bigint)` | 2115 | 20 | 20 (`int8`) |
| `max(text)` | 2129 | 25 | 25 (`text`) |
| `max(numeric)` | 2130 | 1700 | 1700 (`numeric`) |

This is a feasibility GO only. A later implementation task must still audit
semantic effects, dependency completeness, and manifest entries. Exact OID,
namespace, arity, argument/result types, and session binding remain required;
name or volatility alone is not proof.

### MySQL 8.0 and TiDB — DEFER under the kill criterion

The MySQL 8.0.45 probe established that stored functions can be declared
`DETERMINISTIC`, but determinism is not a trust root. A builtin-like
`CREATE FUNCTION count(...)` attempt failed with syntax error, while a
user-defined `my_sum` function was accepted. `information_schema.ROUTINES`
exposes stored functions by schema and `mysql.func` can hold loadable UDFs, but
there is no offline OID-equivalent identity binding for builtins. Therefore any
static `SUM`/`COUNT` promotion would be a name/determinism allowlist and hits
the kill criterion.

TiDB is a separate proof domain despite parser-adjacent sharing with MySQL. No
version-scoped, shadowing-safe offline builtin identity was established. The
same name-only allowlist kill criterion applies.

Both dialects keep `unknown_function_effect`; neither receives promotion in
Phase 1. A future provider would require a separately designed, audited
identity model and caller-session surface.

### Task 7/8 disposition

PostgreSQL is the only dialect promoted for the Phase 1 subset, and only through
the trusted SDK session path. The default SDK, CLI, and HTTP paths remain
`indeterminate` for function-bearing PostgreSQL SQL. MySQL and TiDB are
explicitly deferred: their aggregate and window functions remain
`indeterminate` with `unknown_function_effect` on every current surface. The
kill criterion is unchanged—without offline OID-equivalent identity binding or
a separately designed caller-session proof, a name or determinism allowlist is
not admissible evidence. No CLI/HTTP database connection path is added, and no
MCP Query Access tool is added.

## Consequences

The milestone favors a small useful set of reporting queries over a broad but
unverifiable function allowlist. It may stop one dialect without blocking a
separately proven dialect. This decision remains `Proposed` until proof,
corpus, public-surface, and audit evidence exists in Task 9.

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
