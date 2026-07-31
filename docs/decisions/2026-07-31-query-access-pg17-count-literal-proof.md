# Decision: PG17 Query Access `COUNT(1)` Proof

- Date: 2026-07-31
- Status: Proposed
- Baseline: `main@d7852b8`
- Related: [common pure-effect admissibility](2026-07-16-query-access-common-pure-effects.md), [MySQL/TiDB literal-only and reversed operands](2026-07-26-query-access-literal-only-and-reversed-operands.md)
- Spec: `docs/plans/2026-07-31-query-access-pg17-count-literal-proof-spec.md`
- Design: `docs/plans/2026-07-31-query-access-pg17-count-literal-proof-design.md`
- Implementation: `docs/plans/2026-07-31-query-access-pg17-count-literal-proof-implementation.md`

## Context

The trusted PG17 Query Access SDK already proves bounded aggregates through a
caller-owned session. `COUNT(1)` remains indeterminate because its argument is
a literal, even though the query has a physical base-relation read. MySQL/TiDB
support for literal operands does not establish PostgreSQL aggregate identity,
argument coercion, or session-bound catalog proof.

## Proposed Decision

Evaluate, and implement only if the evidence is sufficient, a single PG17
shape: `COUNT(1)` over a schema-qualified resolved physical base relation on
the existing caller-owned trusted SDK session.

The proof must bind the selected aggregate to facts from the connected PG17
catalog. A successful analysis returns `read_only` and `admissible` with the
base relation's `read_table` requirement. The literal contributes no object
requirement. This remains static requirement analysis, not an authorization or
execution decision.

## Boundaries

- No global PostgreSQL Phase-1 literal widening and no reuse of the MySQL/TiDB
  profile manifest as PostgreSQL proof.
- No relationless `SELECT COUNT(1)`, scalar literal functions, binary literal
  functions, parameters, casts, expressions, DISTINCT, FILTER, ordering,
  windows, nested calls, or arbitrary arity.
- No default/offline SDK, CLI, HTTP, MCP, MySQL, or TiDB behavior change.
- No public result field, authorization/grant/RLS/masking decision, SQL
  execution, result retrieval, or disclosure of SQL literals, catalog data,
  connection data, credentials, or raw driver errors.

## Alternatives

### Treat every PostgreSQL literal as Phase-1 eligible

Rejected. Parser classification alone cannot prove the resolved catalog
aggregate identity or preserve the fail-closed boundary for casts, parameters,
and expressions.

### Copy the MySQL/TiDB exact-shape manifest

Rejected. PostgreSQL uses a caller-owned PG17 catalog-bound proof model; a
profile/name allowlist would weaken that model.

### Keep `COUNT(1)` indeterminate

Retained if catalog evidence cannot prove the exact shape. The milestone must
defer rather than approximate.

## Acceptance Evidence

This record may become Accepted only after parser, catalog/session, strict
requirements, corpus, SDK Docker, no-execution, no-leak, and fail-closed
regressions pass; all affected default and PostgreSQL-tagged gates pass; and
an independent review finds no P0/P1/P2 issue. Until then, `COUNT(1)` remains
indeterminate on PostgreSQL and this decision remains Proposed.
