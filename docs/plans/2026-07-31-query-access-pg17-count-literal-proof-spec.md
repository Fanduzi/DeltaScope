# Spec: PG17 Query Access `COUNT(1)` Proof

## Status

Proposed. This document defines a narrow product change for later implementation;
it does not change current Query Access behavior.

## Problem

The trusted PostgreSQL Query Access SDK can prove `COUNT(*)` and
`COUNT(base_column)` on a caller-owned PG17 connection. It keeps
`COUNT(1)` indeterminate because the argument is a literal rather than a
resolved physical column. MySQL/TiDB already have a separate, profile-bound
proof model for this shape, but that model is not evidence for PostgreSQL.

For reporting queries, `COUNT(1) FROM app.orders` reads the base relation but
does not add a column requirement. It should be usable only if the existing
PostgreSQL session proof can establish the actual PG17 aggregate identity and
all normal relation requirements are complete.

## Objective

Admit exactly this shape through the existing caller-owned PG17 SDK path:

```sql
SELECT COUNT(1) FROM app.orders
```

On a successful proof, the result is `read_only` and `admissible` with exactly
one requirement:

```text
app.orders / read_table
```

The literal contributes no table or column requirement. The result remains a
static requirement analysis, not authorization or execution permission.

## Required Contract

1. The public entry point remains
   `NewPostgreSQLQueryAccessSessionFromConn` plus
   `AnalyzePostgreSQLQueryAccessWithSession`, built with `-tags postgresql`.
2. The candidate must be exactly unqualified aggregate `COUNT` with one
   constant operand, no `DISTINCT`, `FILTER`, local ordering, `OVER`, cast,
   parameter, nested expression, alias-qualified call, or additional argument.
3. The query must contain at least one schema-qualified, uniquely resolved
   physical base relation. `SELECT COUNT(1)` without `FROM` remains
   indeterminate.
4. The existing strict physical-requirements proof must remain unchanged. A
   relation, view, CTE, derived table, wildcard, ambiguous relation, missing
   metadata, or unresolved input remains indeterminate.
5. The connected server must be the existing supported PostgreSQL 17 identity,
   and the proof must bind the aggregate to the session's actual catalog facts.
   Function spelling, parser shape, volatility text, or a hard-coded OID alone
   is insufficient.
6. Default/offline SDK, CLI, HTTP, MCP, MySQL, and TiDB behavior is unchanged.
   This milestone neither opens a connection for those surfaces nor adds an MCP
   Query Access tool.
7. The analyzer must not execute user SQL. User literal text, SQL text,
   connection details, credentials, catalog identifiers, and raw driver errors
   must not leak through result objects, JSON, CLI output, HTTP output, or logs.

## Explicit Non-Goals

- No broad PostgreSQL literal support.
- No `LOWER('x')`, `COALESCE`, `NULLIF`, arithmetic, operators, windows, or
  date/time functions.
- No `COUNT($1)`, `COUNT('1'::int)`, `COUNT(1 + 0)`, `COUNT(DISTINCT 1)`,
  `COUNT(1) FILTER (...)`, `COUNT(1) OVER (...)`, or `COUNT(1, 2)`.
- No relationless promotion, authorization/grant evaluation, RLS/masking,
  SQL rewriting, query execution, result retrieval, or public JSON field.

## Acceptance Evidence

Implementation may change the ADR to Accepted only after all evidence exists:

- Parser characterization proves the admitted and every excluded operand shape.
- Session/catalog tests prove the actual PG17 aggregate identity for `COUNT(1)`
  and reject identity, version, and type-resolution mismatches.
- Unit, corpus, and public SDK tests prove the exact `read_table` requirement,
  deterministic reason codes, and fail-closed regressions.
- Docker-backed PG17 integration proves the caller-owned SDK path and confirms
  no user SQL executes.
- No-leak tests cover marker literals in public results and diagnostics.
- Full PostgreSQL-tagged test, race, build, vet, corpus, formatting, and
  decision-record gates pass.
