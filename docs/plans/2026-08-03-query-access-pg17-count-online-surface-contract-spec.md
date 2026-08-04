# Spec: PG17 `COUNT(1)` Online Surface Contract

## Status

Proposed. This document defines evidence required before the accepted PG17
`COUNT(1)` proof may become a supported online CLI or HTTP contract. It does
not change current behavior or expand a public promise.

## Problem

The accepted PG17 `COUNT(1)` decision proves one exact statement through the
caller-owned SDK session. The existing online CLI and HTTP handlers already
open a PostgreSQL session and call the same public session API, but source-code
wiring alone is not evidence that either transport is a safe, supported surface
for this literal shape. In particular, the CLI owns a direct connection and
HTTP selects an operator-managed `connection_id`; their credential, lifecycle,
error, and logging boundaries must be verified separately.

## Objective

If and only if the existing online paths pass the evidence below, publish the
same exact PG17 statement envelope on three online surfaces:

```sql
SELECT COUNT(1) FROM app.orders
```

The only successful result is `read_only` plus `admissible`, with exactly:

```text
app.orders / read_table
```

The proof must use one connected PostgreSQL 17 instance and the existing
catalog-bound session proof. No new flag, request field, result field, profile,
or parser/manifest shape is in scope.

## Required Contract

1. The SDK contract remains the accepted caller-owned PG17 session contract.
2. CLI support, if promoted, is only online `query-access analyze` using its
   existing local PostgreSQL connection options. It must pass the pinned
   connection to `NewPostgreSQLQueryAccessSessionFromConn` and close the
   CLI-owned connection after analysis.
3. HTTP support, if promoted, is only `POST /v1/query-access/analyze` with an
   operator-configured PostgreSQL 17 `connection_id` authorized for
   `query_access`. The request cannot supply endpoint, credential, secret
   source, TLS, server version, dialect profile, or a direct connection object.
4. Both transports must call
   `AnalyzePostgreSQLQueryAccessWithSession` on the same pinned connection
   used for server identity and metadata. They must not create a parallel
   `COUNT(1)` proof or execute submitted SQL.
5. The exact statement remains restricted to one schema-qualified resolved
   physical base table, no referenced columns, and no additional clauses.
   Every shape deferred by the accepted SDK decision remains indeterminate.
6. CLI without connection options and HTTP without `connection_id` remain
   offline and indeterminate for this function-bearing query. PostgreSQL
   versions other than 17, failed identity/catalog proof, and unavailable or
   unauthorized connections remain bounded failures or indeterminate results
   according to their existing transport contracts.
7. Public CLI stdout/stderr, HTTP bodies, HTTP access logs, and SDK-compatible
   result JSON must not expose submitted SQL, literal markers, endpoint,
   connection ID, username, password, password-source name or file path,
   server version, catalog identifiers, backend details, or raw driver errors.
8. This remains static requirement analysis, not SQL execution, query result
   retrieval, authorization, grants, RLS, masking, or an execution guarantee.

## Explicit Non-Goals

- No broader PostgreSQL literal support.
- No `COUNT(NULL)`, `COUNT(2)`, `COUNT('1')`, casts, parameters, expressions,
  DISTINCT, FILTER, ordering, windows, joins, CTEs, derived relations, views,
  unqualified relations, relationless queries, or additional clauses.
- No default/offline promotion, MCP Query Access tool, new configuration
  surface, direct HTTP credentials, public proof metadata, or error payload.
- No claim that existing source-code delegation is public support until the
  required live transport evidence passes.

## Acceptance Evidence

The successor ADR may change from Proposed to Accepted only after:

- Docker-backed PG17 CLI E2E invokes the built CLI through its real online
  connection flags and proves the positive statement, exact requirement,
  excluded-shape indeterminacy, and offline regression.
- Docker-backed HTTP E2E invokes the real handler/server with an
  operator-managed `connection_id` and proves the same result, authorization
  boundary, excluded-shape indeterminacy, and no-connection default.
- The shared-session recording-driver proof remains required, but it cannot
  substitute for adapter-level evidence. CLI online and HTTP
  `connection_id` paths each require an observable transport-level test seam.
  The seam may be a test-only injected opener/dialer, recording driver, or
  controlled proxy chosen by the implementation, provided it observes database
  operations before and after the shared session boundary without defining a
  new production API contract.
- For each successful online path, the transport-level test must observe at
  least one expected fixed identity/catalog probe and must prove that the
  submitted SQL's unique marker, `EXPLAIN`, and prepare operations never reach
  the driver or proxy. The fixed-probe observation prevents a no-execution
  assertion from passing vacuously.
- For HTTP rejected or unauthorized `connection_id` paths, the transport-level
  test must assert zero dial/open-session operations and no leakage of
  connection configuration or credentials.
- These adapter-level CLI and HTTP proofs, together with the shared-session
  proof, are mandatory before this ADR may change from Proposed to Accepted.
- Marker-based no-leak tests cover normal and reachable connection/catalog
  failure paths across CLI stdout/stderr, HTTP response/access log, and public
  result serialization.
- Existing `COUNT(*)` and `COUNT(column)` PostgreSQL proof regressions remain
  admissible, including the repaired generic `COUNT(column)` path.
- PostgreSQL-tagged suites, focused race tests, Docker evidence, corpus and
  release gates, formatting, module-tidy, and an independent read-only review
  all pass with no P0/P1/P2 findings.
