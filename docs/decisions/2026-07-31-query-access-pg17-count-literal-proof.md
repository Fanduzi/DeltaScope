# Decision: PG17 Query Access `COUNT(1)` Proof

- Date: 2026-07-31
- Status: Accepted
- Related milestone/version: v0.480.0
- Baseline: `main@d7852b8`
- Related: [common pure-effect admissibility](2026-07-16-query-access-common-pure-effects.md), [MySQL/TiDB literal-only and reversed operands](2026-07-26-query-access-literal-only-and-reversed-operands.md)
- Spec: `docs/plans/2026-07-31-query-access-pg17-count-literal-proof-spec.md`
- Design: `docs/plans/2026-07-31-query-access-pg17-count-literal-proof-design.md`
- Implementation: `docs/plans/2026-07-31-query-access-pg17-count-literal-proof-implementation.md`

## Context

The trusted PG17 Query Access SDK already proves bounded aggregates through a
caller-owned session. `COUNT(1)` required a separate proof because its argument
is a literal, even though the query has a physical base-relation read. MySQL/TiDB
support for literal operands does not establish PostgreSQL aggregate identity,
argument coercion, or session-bound catalog proof.

## Decision

Support a single PG17 statement envelope: `COUNT(1)` over exactly one
schema-qualified resolved physical base relation, with no referenced columns or
additional clauses, on the existing caller-owned online SDK session.

The parser may contribute only a non-serialized `integer_one` syntax fact for
an uncast AST integer constant whose parsed value is `1`; it must not retain
the literal text. A narrow Phase-1 eligibility exception may recognize that
fact, but it cannot promote a result. Promotion requires a session-bound PG17 catalog proof of the
`pg_catalog.count(any)` aggregate identity. A successful analysis returns
`read_only` and `admissible` with the sole base relation's `read_table`
requirement. This remains static requirement analysis, not an authorization or
execution decision.

## Boundaries

- No global PostgreSQL Phase-1 literal widening and no reuse of the MySQL/TiDB
  profile manifest as PostgreSQL proof.
- No promotion based on parser `const` alone, a fabricated literal type OID, a
  literal treated as a column, a hard-coded OID, or a name-only lookup.
- No relationless `SELECT COUNT(1)`, scalar literal functions, binary literal
  functions, `COUNT(NULL)`, `COUNT(2)`, `COUNT('1')`, parameters, casts,
  expressions, joins, comma joins, DISTINCT, FILTER, ordering, windows, nested
  calls, or arbitrary arity.
- No default/offline SDK, CLI, HTTP, MCP, MySQL, or TiDB behavior change.
- No public result field, authorization/grant/RLS/masking decision, SQL
  execution, result retrieval, or disclosure of SQL literals, catalog data,
  connection data, credentials, or raw driver errors.

## Alternatives

### Treat every PostgreSQL literal as Phase-1 eligible

Rejected. Parser classification alone cannot prove the resolved catalog
aggregate identity or preserve the fail-closed boundary for casts, parameters,
and expressions.

### Treat every parser `const` as the integer literal `1`

Rejected. The current parser intentionally records only a broad `const` kind.
The proposed internal `integer_one` fact must distinguish the one admitted
syntax form without retaining or exposing literal text; otherwise the milestone
remains deferred.

### Copy the MySQL/TiDB exact-shape manifest

Rejected. PostgreSQL uses a caller-owned PG17 catalog-bound proof model; a
profile/name allowlist would weaken that model.

### Keep `COUNT(1)` indeterminate

Retained if catalog evidence cannot prove the exact shape. The milestone must
defer rather than approximate.

## Verification Evidence

- The repository `docker/pg-e2e-compose.yaml` and
  `docker/postgresql/init.sql` provide a reproducible PostgreSQL 17 fixture.
- Parser, Phase-1, catalog/session, requirements, and caller-owned SDK
  integration tests prove the exact envelope and excluded shapes.
- The session-bound dedicated `pg_catalog.count(any)` lookup uses an atomic
  catalog snapshot and does not execute user SQL.
- Recording-driver tests prove no-execution and no-leak behavior for both
  successful proof and dedicated catalog lookup failure, including JSON,
  struct, and driver boundaries.
- Independent read-only audit found no P0, P1, or P2 issues.

## Deferred Scope

- PostgreSQL support remains limited to the exact
  `SELECT COUNT(1) FROM <single schema-qualified physical table>` shape on a
  caller-owned online PostgreSQL 17 SDK session.
- Other PostgreSQL literal/count shapes remain indeterminate, including
  `COUNT(NULL)`, other literals, casts, parameters, modifiers, joins, CTEs,
  derived relations, views, unqualified or relationless forms, and additional
  clauses.
- Default offline SDK behavior and CLI, HTTP, and MCP surfaces are not
  expanded.
- This is static requirement analysis, not an authorization, SQL execution,
  result retrieval, RLS, grant, or masking decision.
