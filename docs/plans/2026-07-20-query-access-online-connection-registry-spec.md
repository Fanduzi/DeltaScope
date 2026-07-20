# Specification: Online Query Access Connections and Common Scalar Effects

Date: 2026-07-20
Status: Proposed
Decision: `docs/decisions/2026-07-20-query-access-online-connection-registry.md`
Baseline: `main@49b6fae`

## Product Requirement

Query Access should be able to analyze a bounded set of ordinary function
queries when an operator intentionally provides database access. It must
identify the actual server, resolve physical tables and columns, and return
static read requirements. It must never execute the SQL being analyzed or
return query data.

CLI keeps its local direct-connection workflow. HTTP changes from a caller
supplied direct connection to an operator-managed named connection. This fixes
the current boundary where an HTTP caller can influence server-side credential
resolution and outbound database targets.

## User Contract

### CLI

`deltascope query-access analyze` has two modes:

| Mode | Input | Behavior |
| --- | --- | --- |
| Offline | SQL plus `--dialect` | Does not connect. Function-bearing SQL remains `indeterminate`. |
| Online | SQL plus local `--host`, `--port`, `--user`, and password/`--ask-password` flags | Opens a short-lived connection, identifies the server, resolves metadata, analyzes, then closes its connection. |

Online CLI mode derives its dialect and capability target from the connected
server. `--profile` is removed. An explicit `--dialect`, if retained, is only a
consistency assertion and mismatches fail with a bounded error.

### HTTP

Both `POST /v1/audit` and `POST /v1/query-access/analyze` accept a configured
connection ID for online work:

```json
{
  "connection_id": "orders-prod",
  "sql": "SELECT LOWER(o.email) FROM orders.orders AS o",
  "mode": "strict"
}
```

The legacy `connection` object is removed without a migration switch. HTTP
rejects request-local host, port, socket, user, password, password source, TLS,
profile, and server-version fields before secret resolution or a network dial.

### Operator Configuration

Runtime configuration owns the connection inventory. Each entry has a stable
ID, engine/dialect, endpoint, metadata account, schema policy, secret source,
TLS mode, purpose allowlist, and API-key allowlist. The HTTP request can select
only the ID.

```yaml
http:
  auth:
    keys:
      - id: orders-reviewer
        secret_env: DELTASCOPE_ORDERS_REVIEWER_API_KEY
metadata:
  connections:
    - id: orders-prod
      dialect: mysql
      host: mysql-orders.internal
      port: 3306
      user: deltascope_metadata
      schema: orders
      password_env: DELTASCOPE_ORDERS_PROD_PASSWORD
      tls_mode: disabled
      purposes: [audit, query_access]
      allowed_api_key_ids: [orders-reviewer]
```

`password_env`, `password_file`, API-key environment names, and API-key files
are allowed only in operator configuration. Multiple databases normally have
multiple entries and can use different credentials. `tls_mode` defaults to
`disabled`; when `enabled`, certificate and hostname validation are mandatory.

With HTTP authentication enabled, a connection is usable only by its explicitly
listed API-key identities. With authentication disabled, the deployment is
treated as trusted self-hosted and all configured connections are usable.

## Online Analysis Boundary

Online Query Access uses a dedicated low-privilege metadata account. It executes
only fixed server-identity and catalog/metadata queries. It does not execute,
prepare, explain, or return the submitted SQL.

The same pinned connection must identify the server and resolve relation/column
metadata. Supported series are:

| Connected server | Derived target |
| --- | --- |
| MySQL `5.7.x` | MySQL 5.7 |
| MySQL `8.0.x` | MySQL 8.0 |
| MySQL `8.4.x` | MySQL 8.4 |
| TiDB `8.5.x` | TiDB 8.5 |
| PostgreSQL 17 | existing PostgreSQL 17 trusted path |

Unknown products, forks, malformed identity, unsupported version series, and
dialect disagreement return bounded errors. Patch versions are accepted within
the supported major/minor series. The existing public analysis-profile field is
deprecated for source compatibility; it cannot promote an offline query and is
not accepted for online CLI/HTTP/session analysis.

Supported native forms do not require SQL-mode attestation. `COUNT(*)` is
eligible when otherwise proven; `COUNT (*)` and comment-separated variants are
not. The latter remain `indeterminate`.

## Common Scalar Candidate Scope

The following are candidates, not an automatic allowlist. Every dialect and
server series requires independent primary-source evidence, immutable manifest
or catalog proof, parser facts, and live Docker E2E before admission:

| Family | Candidate calls | Operand rule |
| --- | --- | --- |
| String | `LOWER`, `UPPER`, `LENGTH`, `CHAR_LENGTH` | every argument is a direct physical base column |
| Numeric | `ABS`, `CEIL`, `FLOOR` | every argument is a direct physical base column |
| Null handling | `COALESCE`, `NULLIF`, and native `IFNULL` | every argument is a direct physical base column |

Candidates may appear in projection, `WHERE`, `JOIN ON`, `GROUP BY`, `HAVING`,
or `ORDER BY`. All direct input columns become strict requirements. PostgreSQL,
MySQL, and TiDB remain independent proof domains; `IFNULL` is never implied for
PostgreSQL by MySQL/TiDB support.

## Fail-Closed Rules

The following remain normal `indeterminate` results: unqualified relations,
views, wildcards, missing metadata, quoted or schema-qualified calls,
noncanonical spacing/comments, unknown/UDF/stored/plugin functions, literals,
parameters, `NULL`, casts, arithmetic, nesting, subqueries, `DISTINCT`,
`FILTER`, aggregate-local ordering, named windows, frames, unknown AST nodes,
and any query containing an unproven candidate.

Write statements, locking reads, `SELECT INTO`, external-file forms, and
multi-statements retain rejected/not-read-only behavior. Setup, identity,
authorization, and configuration failures return bounded errors instead.

`admissible` remains a static requirement result. It is not database
authorization, grant/RLS/masking/rewrite evaluation, a query result, or an
execution-snapshot guarantee.

## Non-Goals

- Executing SELECT or returning data.
- Generic name allowlists, arbitrary UDF/stored function support, or caller
  supplied manifests/profiles/version assertions.
- SQL-mode attestation, arbitrary forks/releases, casts, literals, parameters,
  nested expressions, JSON/regex/date functions, views, or broad SELECT support.
- HTTP direct credentials or HTTP caller-controlled server environment/file
  access.
- An MCP Query Access tool.

## Exit Criteria

The decision can be Accepted only after HTTP audit and Query Access both use
named connections; identity, authorization, TLS, no-leak, and no-execution
tests pass; every shipped function has per-dialect live evidence; online SDK,
CLI, and HTTP agree; default offline behavior remains fail-closed; and security
and plan reviews close all P1/P2 findings.
