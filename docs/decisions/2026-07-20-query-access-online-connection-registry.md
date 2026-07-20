# Decision: Online Query Access Uses Operator-Managed Named Connections

- Date: 2026-07-20
- Status: Proposed
- Baseline: `main@49b6fae`
- Related: [Query Access foundation](2026-07-11-query-access-analysis-foundation.md), [pure-read admissibility](2026-07-12-query-access-pure-read-admissibility.md), [common pure effects](2026-07-16-query-access-common-pure-effects.md), [MySQL/TiDB builtin semantic manifests](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md)

## Context

Query Access has online SDK session paths, but CLI and HTTP remain offline for
function-bearing SELECT analysis. This excludes useful analysis even when an
operator intentionally wants DeltaScope to inspect a real instance's version
and physical metadata.

The current HTTP audit endpoint accepts a direct connection object. It permits
caller-selected outbound targets and server-side password-source fields such as
environment variables and file paths. Redacting responses would not make that a
sound production boundary because the request could still direct secret
resolution and network activity.

The project needs an open-source-friendly solution that works with local config
files and environment variables, without requiring an enterprise secret manager,
while allowing larger deployments to restrict each database connection.

## Decision

HTTP audit and Query Access will use one operator-managed named connection
registry. HTTP requests carry `connection_id`, never endpoint, credential,
secret source, socket, TLS setting, profile, or version declaration. Runtime
configuration owns those values.

Operator configuration may reference environment variables or permission-
restricted files for database passwords and API keys. HTTP cannot name an
environment variable or file. The direct HTTP `connection` object is removed
without a compatibility switch.

CLI remains a local tool and may use direct connection flags. The CLI owns its
connection and never forwards its credentials to HTTP.

When HTTP authentication is enabled, configured API keys have stable internal
IDs. Each connection lists permitted key IDs and permitted purposes: `audit`,
`query_access`, or both. Access is deny-by-default. When authentication is
disabled, the deployment is treated as trusted self-hosted and every configured
connection is usable.

Each online session uses the same pinned connection to identify the actual
database product/release series and resolve physical table/column metadata.
MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17 are the only candidate series.
The session, not the caller, derives the internal capability target. Unknown
products, forks, malformed identity, unsupported versions, and dialect
disagreement return bounded errors. Patch versions are accepted within a proven
major/minor series.

Online Query Access never executes submitted SQL. It uses only fixed identity
and metadata/catalog queries. The result remains a static requirement report,
not data results or authorization.

The milestone also evaluates a narrow scalar candidate set: `LOWER`, `UPPER`,
`LENGTH`, `CHAR_LENGTH`, `ABS`, `CEIL`, `FLOOR`, `COALESCE`, `NULLIF`, and
dialect-specific `IFNULL`. Every dialect/version needs independent evidence
before admission. Every argument must be a direct physical base column and is a
strict requirement.

## Rationale

Named connections prevent HTTP callers from choosing where the service dials or
what server-local secret source it reads. A small deployment can configure a
few local connection/password references; a larger deployment can use the same
model with a secret manager or deployment system.

One registry for DML/DDL audit and Query Access prevents divergent security
models. Same-connection identity makes MySQL/TiDB semantic profiles truthful
and retains PostgreSQL's session-bound proof model.

The scalar subset covers common expressions while refusing to guess about
literals, casts, nesting, user-defined functions, and unproven dialect behavior.

## Public Contract

- HTTP online requests use `connection_id`; direct connection fields are
  rejected.
- CLI supports local direct connection flags; no CLI `--profile` is exposed.
- Online SDK, CLI, and HTTP derive capability from connected-server identity;
  callers cannot select a profile for promotion.
- Default offline SDK remains offline and function-bearing SQL remains
  `indeterminate` there.
- With HTTP authentication, connections require explicit principal allowlists.
- `tls_mode` defaults to `disabled`; `enabled` requires certificate and hostname
  verification and has no insecure mode.
- Public outputs, errors, and logs are bounded: no secrets, endpoints, server
  identity, raw SQL, or internal proof data.
- MCP remains without a Query Access tool.

## Deferred Scope

This decision does not add SQL execution or data-returning APIs; arbitrary host/
password HTTP requests; user-selected server files/environment variables; a
mandatory secret manager; SQL-mode attestation; unsupported database forks or
releases; arbitrary functions/UDFs; casts, literals, parameters, nested
expressions, JSON/regex/date functions, views, broad SELECT support; grants,
RLS, masking, rewrite, execution-snapshot guarantees, or authorization checks.

## Acceptance Evidence

This record can change to `Accepted` only after:

1. HTTP audit and Query Access reject all legacy direct connection fields before
   secret resolution or dialing.
2. Runtime configuration validates named connections/API keys, purpose and
   principal authorization, TLS, and secret sources without public leakage.
3. SDK, CLI, and HTTP use the same online session factory and never execute
   submitted SQL.
4. Claimed MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17 support has
   non-skippable Docker evidence; unsupported identity is bounded.
5. Every admitted scalar entry has independent documentation, proof, corpus,
   dependency, and live E2E evidence.
6. Success/error/log no-leak tests cover injected SQL, literal, credential,
   source-path, endpoint, identity, candidate, manifest, and driver markers.
7. Existing audit, offline SDK, CLI, HTTP, MCP, PostgreSQL, function-free
   MySQL/TiDB, and aggregate/window behavior has regression coverage.
8. Oracle/Momus P1/P2 findings are closed, or unavailable review is recorded
   honestly with an equivalent adversarial audit.

## Consequences

HTTP clients must move to administrator-provisioned IDs. This intentional
breaking change removes an unsafe connection/credential boundary. Runtime
configuration becomes a public server contract and requires clear startup
validation and operational documentation.
