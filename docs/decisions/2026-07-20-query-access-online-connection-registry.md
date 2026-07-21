# Decision: Online Query Access Uses Operator-Managed Named Connections

- Date: 2026-07-20
- Status: Accepted
- Baseline: `main@6903364`
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

This record changed to `Accepted` after the following evidence was collected:

1. **Legacy field rejection**: `POST /v1/audit` and `POST /v1/query-access/analyze`
   reject `connection` object and all direct endpoint/credential fields with HTTP
   400 before any secret resolution or dialing. Verified by
   `TestAuditRequestRejectsLegacyConnectionField` and
   `TestQueryAccessRejectsProfileWithConnectionID`.

2. **Runtime configuration validation**: `ValidateAndBuildRegistry()` validates
   connection IDs, dialects, purposes, TLS modes, endpoints, secret sources,
   and API-key references at startup. Missing env vars/files fail with bounded
   messages (no secret names or values in errors). Verified by 60 table-driven
   tests in `registry_test.go`.

3. **Shared session factory**: SDK, CLI, and HTTP all use
   `online.OpenSession()` which pins one `*sql.Conn`, captures server identity
   via `SELECT VERSION()`, and derives capability from actual product/version.
   Submitted SQL is never sent to the driver. Verified by recording-driver
   tests in `session_test.go` and identity parser tests in `identity_test.go`.

4. **Docker evidence**: MySQL 5.7.44, MySQL 8.0.46, MySQL 8.4.10, TiDB 8.5.7,
   and PostgreSQL 17 are exercised by `docker/query-access-builtin-compose.yaml`
   and `docker/pg-e2e-compose.yaml`. Unsupported identity (MariaDB, MySQL 5.6,
   PG 16) returns bounded `ErrIdentityUnsupported`. Verified by integration
   tests in `builtin_semantic_live_probes_test.go` and
   `effect_identity_resolver_integration_test.go`.

5. **Scalar proof**: LOWER, UPPER, LENGTH, CHAR_LENGTH, ABS, CEIL, FLOOR,
   COALESCE, NULLIF (and IFNULL for MySQL/TiDB) each have independent manifest
   entries, parser-native-form facts, corpus fixtures, and live Docker E2E
   evidence per dialect/version. Every scalar function requires all operands to
   be direct physical base columns; `IFNULL(column, literal)` and
   `COALESCE(column, literal)` are indeterminate. PG17 uses catalog-bound
   OID/type/volatility proof. MySQL/TiDB use versioned native-form semantic
   manifests. Verified by `TestBuiltinSemanticProfileRegression*` and
   `TestScalarLive_*` tests.

6. **No-leak coverage**: Injected markers (passwords, DSNs, hostnames, ports,
   driver errors, SQL literals, API keys, version strings, manifest data,
   candidates) are verified absent from HTTP responses, CLI stdout/stderr, and
   access logs. Verified by `TestNoLeak*` tests across handler, query-access,
   and probe-boundary test files.

7. **Regression coverage**: 3707 non-PG tests, 4935 PG-tagged tests, corpus
   gates (`make query-access-corpus-gates`), PG unit gates
   (`make pg-unit-test-gates`), npm MCP tests (15/15), build/vet clean for both
   build tags, `go mod tidy` clean, `git diff --check` clean.

8. **Oracle/Momus review**: Oracle security review returned PASS WITH FINDINGS
   (1 LOW: CLI lacks `--tls-mode` flag — deferred per spec which lists only
   `--host`, `--port`, `--user`, `--password-env`, `--password-file`,
   `--ask-password`). Momus plan/diff review returned [OKAY]. No P1/P2 findings
   remain.

9. **Auth-disabled mode**: With `http.auth.enabled: false`, HTTP endpoints accept
   requests without `X-API-Key`; connection existence and purpose checks still
   apply. With auth enabled, missing/invalid/unauthorized keys remain denied.
   Route-level evidence proves `connection_id` requests reach the
   registered-connection path, not merely the offline path. Wrong-purpose
   requests remain denied with `purpose_not_allowed`. Verified by
   `TestHandlerAuditConnectionIDWorksWithAuthDisabled`,
   `TestHandlerAuditConnectionIDWrongPurposeDeniedWithAuthDisabled`,
   `TestHandlerAuditAllowsRequestWithoutAPIKeyWhenAuthDisabled`, and
   `TestHandlerQueryAccessAllowsRequestWithoutAPIKeyWhenAuthDisabled`.

10. **Database and TLS propagation**: `runtimeconfig.ConnectionConfig.Database`
    and `TLSMode` are threaded through `auditmeta.ConnectionConfig` to
    `mysqlmeta.ConnectionConfig` and `postgresqlmeta.ConnectionConfig`. PostgreSQL
    audit connects to the configured database (never silently defaults to
    `postgres`). `tls_mode: enabled` enforces certificate and hostname validation
    (`InsecureSkipVerify=false` for MySQL, `sslmode=verify-full` for PostgreSQL).
    Verified by `TestConnectionConfigDSN*` tests and dedicated TLS E2E fixture
    (`docker/tls-e2e/`) with trusted CA and hostname-valid certificates.

11. **CLI database selection**: `query-access analyze --database` flag allows
    PostgreSQL users to select the target database. The flag is threaded through
    `auditConnectionOptions` to `online.SessionConfig.Database`. Without the
    flag, PostgreSQL defaults to `postgres`. Verified by
    `TestQueryAccessAnalyzeHelpShowsConnectionFlags` and CLI E2E case 16/17
    proving non-default database selection and default-database negative control.

### Deferred Items

- CLI `--tls-mode` flag (Oracle LOW finding): CLI users connecting to remote
  databases cannot enable TLS. Deferred because the spec lists only local direct
  connection flags and CLI typically connects to local databases. Password can
  be piped via `--password-env` or `--ask-password`.

## Consequences

HTTP clients must move to administrator-provisioned IDs. This intentional
breaking change removes an unsafe connection/credential boundary. Runtime
configuration becomes a public server contract and requires clear startup
validation and operational documentation.
