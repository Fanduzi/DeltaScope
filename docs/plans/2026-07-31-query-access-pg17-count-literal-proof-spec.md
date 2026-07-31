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

Admit exactly this statement envelope through the existing caller-owned PG17
SDK path:

```sql
SELECT COUNT(1) FROM app.orders
```

The statement has one `COUNT(1)` target, exactly one schema-qualified physical
base table, and no joins, additional `FROM` items, CTEs, derived sources,
column references, predicates, grouping, ordering, limits, set operations, or
other expression-bearing clauses. On a successful proof, the result is
`read_only` and `admissible` with exactly one requirement:

```text
app.orders / read_table
```

The literal contributes no table or column requirement. The result remains a
static requirement analysis, not authorization or execution permission.

## Required Contract

1. The public entry point remains
   `NewPostgreSQLQueryAccessSessionFromConn` plus
   `AnalyzePostgreSQLQueryAccessWithSession`, built with `-tags postgresql`.
2. The parser must emit an internal-only literal-shape enum for an uncast AST
   integer constant whose parsed value is exactly `1`. It must not retain the
   literal text, and every other constant (including `NULL`, strings, floats,
   signed expressions, and other integers) must have a distinct non-admitted
   shape.
3. The candidate must be exactly unqualified `COUNT` with one
   `integer_one` operand shape, no `DISTINCT`, `FILTER`, local ordering,
   `OVER`, cast, parameter, nested expression, alias-qualified call, or
   additional argument. A parser `const` classification alone is insufficient.
4. The query must contain exactly one schema-qualified, uniquely resolved
   physical base relation and no referenced columns. `SELECT COUNT(1)` without
   `FROM`, comma joins, and explicit joins remain indeterminate.
5. The existing strict physical-requirements proof must remain unchanged. A
   relation, view, CTE, derived table, wildcard, ambiguous relation, missing
   metadata, or unresolved input remains indeterminate.
6. The connected server must be the existing supported PostgreSQL 17 identity,
   and the proof must bind the aggregate to the session's actual catalog facts:
   `pg_catalog`, aggregate class, and its polymorphic `count(any)` signature.
   Function spelling, parser shape, volatility text, or a hard-coded OID alone
   is insufficient. The catalog proof must not pretend that the literal is a
   column or require execution of the submitted SQL.
7. Default/offline SDK, CLI, HTTP, MCP, MySQL, and TiDB behavior is unchanged.
   This milestone neither opens a connection for those surfaces nor adds an MCP
   Query Access tool.
8. The analyzer must not execute user SQL. User literal text, SQL text,
   connection details, credentials, catalog identifiers, and raw driver errors
   must not leak through newly reached SDK result objects, diagnostics, or
   logs. Default CLI and HTTP paths remain unchanged and must retain their
   existing regression coverage.

## Explicit Non-Goals

- No broad PostgreSQL literal support.
- No `LOWER('x')`, `COALESCE`, `NULLIF`, arithmetic, operators, windows, or
  date/time functions.
- No `COUNT($1)`, `COUNT(NULL)`, `COUNT(2)`, `COUNT('1')`,
  `COUNT('1'::int)`, `COUNT(1 + 0)`, `COUNT(DISTINCT 1)`,
  `COUNT(1) FILTER (...)`, `COUNT(1) OVER (...)`, or `COUNT(1, 2)`.
- No joins, comma joins, predicates, grouping, ordering, limits, set
  operations, or any additional table or column reference.
- No relationless promotion, authorization/grant evaluation, RLS/masking,
  SQL rewriting, query execution, result retrieval, or public JSON field.

## Acceptance Evidence

Implementation may change the ADR to Accepted only after all evidence exists:

- Parser characterization proves the internal `integer_one` fact is produced
  only for an uncast AST integer constant whose parsed value is `1`, is never
  serialized, and rejects every excluded operand shape.
- Session/catalog tests prove the actual PG17 `pg_catalog.count(any)` aggregate
  identity and reject identity, version, polymorphic-signature, and
  literal-shape mismatches.
- Unit, corpus, and public SDK tests prove the exact `read_table` requirement,
  deterministic reason codes, and fail-closed regressions.
- Docker-backed PG17 integration proves the caller-owned SDK path and confirms
  no user SQL executes.
- No-leak tests cover marker literals in public results and diagnostics.
- Full PostgreSQL-tagged test, race, build, vet, corpus, formatting, and
  decision-record gates pass.
