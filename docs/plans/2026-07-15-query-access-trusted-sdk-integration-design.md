# Design: Query Access Trusted PostgreSQL SDK Integration

Date: 2026-07-15
Status: Proposed follow-on design
Depends on: `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
Decision status: the pure-read decision remains `Proposed`

## Problem

The pure-read branch contains a PostgreSQL proof gateway, PG17 manifest, and
pinned-session tests. The public SDK entry point, `AnalyzeQueryAccess`, creates
a basic application service and never uses that gateway; CLI and HTTP use the
same default path. This is safe, but it is not a production capability.

The next deliverable is a narrow SDK integration for query platforms that
already own the PostgreSQL connection on which they later execute the query.
It is not a general authorization API and must not make CLI, HTTP, or MCP claim
promotion they cannot bind to an execution session.

## Goals

- Provide an SDK-only path for a caller-owned single PostgreSQL connection to
  obtain the existing manifest-gated analysis result.
- Reuse the one application `Service.Analyze` proof gateway, PG17 manifest,
  metadata resolver, and pinned-session adapter; do not duplicate proof logic.
- Make session ownership and the static-analysis limitation explicit.
- Keep incomplete, changed, unknown, or unsupported conditions indeterminate.
- Preserve default `AnalyzeQueryAccess`, MySQL, and TiDB behavior.

## Non-goals

- Grant evaluation, authentication, execution, transaction management, RLS,
  masking, SQL rewrite, or policy enforcement.
- Claiming a later execution uses the analyzed snapshot.
- Trusted JSON fields for HTTP or CLI, connection-string inputs, or an MCP tool.
- PostgreSQL versions outside the manifest's tested PG17 range.
- Automatic support for unqualified relations, views, casts, UDFs,
  `TABLESAMPLE`, or unsupported AST shapes.

## Public API Shape

Use a separate SDK function rather than putting a session-shaped field on
`QueryAccessRequest`:

```go
func NewPostgreSQLQueryAccessSessionFromConn(conn *sql.Conn) (*PostgreSQLQueryAccessSession, error)

func AnalyzePostgreSQLQueryAccessWithSession(
    ctx context.Context,
    session *PostgreSQLQueryAccessSession,
    req QueryAccessRequest,
) (*QueryAccessResult, error)
```

`PostgreSQLQueryAccessSession` is an opaque public wrapper around the internal
pinned session. It exposes no OIDs, manifest entries, catalog SQL, credentials,
session binding, or `Trusted` flag. `FromConn` does not own or close the
caller's connection. A `*sql.DB` constructor is deferred because DeltaScope
cannot return that allocated connection for subsequent execution without a
separate execution-affinity design.

The function rejects a non-PostgreSQL dialect or nil session with bounded SDK
errors. Catalog failures and non-provable SQL remain normal analysis results
with `admission: indeterminate`; they do not expose driver text.

## Execution Ownership Contract

`admissible` means only that static analysis obtained complete known
requirements and proved the bounded effect manifest against the supplied
connection's catalog context. It is not authorization and does not guarantee a
later execution snapshot.

If a caller uses the result in an execution decision, it must:

1. Keep the same `*sql.Conn` from analysis through authorization and execution.
2. Reanalyze if role, database, server version, or `search_path` may change.
3. Evaluate grants/policy itself and retain database authorization as final
   enforcement.
4. Treat `indeterminate` as ineligible for automatic execution.

DeltaScope cannot enforce same-connection execution after it returns. This is
intentional: accepting a pool, DSN, or generic resolver would create a false
execution-locality claim.

## Internal Wiring

The SDK adapter creates, from the same opaque session:

1. a PostgreSQL metadata `SchemaResolver`;
2. the controlled atomic effect-identity resolver;
3. `NewTrustPolicy(NewPG17Manifest())`; and
4. `NewTrustedService(...)`.

It invokes only `Service.Analyze`. Existing barriers remain authoritative:
unqualified base relations, views, unresolved metadata, wildcard failures,
unsupported traversal, incomplete context/fact pins, candidate/fact/type-map
mismatches, and non-manifest identities all remain fail-closed.

No public caller can inject identity facts, a trust policy, a resolver, OIDs,
or a `Bound` context.

## Admission Matrix

| Input | Trusted SDK result |
| --- | --- |
| PG17, schema-qualified base tables, complete metadata, `count(*)` | May be `read_only` + `admissible` after exact proof |
| PG17, schema-qualified base columns and a manifest comparison | May be `read_only` + `admissible` after exact proof |
| Default SDK/CLI/HTTP/MCP path | Existing fail-closed behavior; no trusted promotion |
| Unqualified relation, view, wildcard failure, unsupported AST, `TABLESAMPLE` | `indeterminate` |
| Literal/parameter/NULL/coercion/cast/UDF/non-manifest effect | `indeterminate` |
| Session/context/type-map/catalog failure or drift | `indeterminate` |

## Privacy and Surface Rules

The result remains `QueryAccessResult`. It must not gain OID, signature,
candidate, manifest, session binding, search path, catalog query, SQL/literal,
credential, connection-string, or `severity` fields. SDK errors stay bounded.

CLI/HTTP may not claim trusted promotion. MCP remains unchanged.

## Go/No-Go Criteria

Proceed only if every internal dependency can be created from the same
caller-owned `*sql.Conn` without exposing it in JSON or falling back to a pool.
Stop and keep the decision `Proposed` if any condition is false:

- caller ownership cannot be preserved without a misleading execution claim;
- trusted service needs public resolver/facts/OID injection;
- PG17 Docker E2E cannot prove the public SDK function uses one connection; or
- the new path makes a default SDK/CLI/HTTP result more permissive.

## Deferred Follow-ups

- An execution callback that can enforce analysis-to-execution affinity needs a
  separate authorization/execution design.
- CLI/HTTP live metadata needs a server-side connection and principal model.
- More PostgreSQL majors or manifest entries require re-probe and a decision
  record amendment.
