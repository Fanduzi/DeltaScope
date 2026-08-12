# Spec: Unified Online Query Access Analysis Entry

## Status

Proposed. This document defines an additive public SDK boundary and a
behavior-preserving CLI/HTTP migration. It does not authorize implementation,
merging, release, or removal of existing APIs or tests.

## Problem

Online Query Access already has one security model: a caller or transport owns
a pinned `*sql.Conn`, DeltaScope identifies the server on that connection, and
all metadata proof stays on the same connection. The public SDK expresses that
model through separate PostgreSQL and MySQL/TiDB session types. CLI and HTTP
therefore both repeat the same product switch, dialect assignment, session
construction, and dialect-specific analysis call.

That duplicated transport wiring has already drifted in small ways. More
importantly, it puts a product-to-proof routing decision in transport adapters
instead of the public Query Access module that owns the proof boundary.

Official DeltaScope CLI, server, and MCP release binaries are all built with
PostgreSQL support. The repository also deliberately compiles and tests the Go
package without the `postgresql` build tag. That source-compatibility path must
not be described as a PostgreSQL-disabled official product, but its public API
shape must remain compilable and fail closed when PostgreSQL capability is not
linked.

## Objective

Add one opaque public online session and one analysis entry that route MySQL
5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17 from identity observed on a
caller-owned pinned connection. Migrate CLI and HTTP to that entry without
moving connection resolution, authorization, lifecycle ownership, or error
presentation into the SDK.

## Proposed Public API

```go
type OnlineQueryAccessSession struct { /* opaque */ }

func NewOnlineQueryAccessSessionFromConn(
    ctx context.Context,
    conn *sql.Conn,
) (*OnlineQueryAccessSession, error)

func AnalyzeOnlineQueryAccessWithSession(
    ctx context.Context,
    session *OnlineQueryAccessSession,
    req QueryAccessRequest,
) (*QueryAccessResult, error)
```

The following public sentinel errors form the generic boundary:

```go
var (
    ErrOnlineQueryAccessSessionUnavailable
    ErrOnlineQueryAccessDialectMismatch
    ErrOnlineQueryAccessProfileNotAllowed
    ErrOnlineQueryAccessSchemaResolverNotAllowed
    ErrOnlineQueryAccessCapabilityUnsupported
)
```

Exact error text is part of implementation review. Public callers must use
`errors.Is`, not string matching.

## Required Contract

1. The constructor accepts only `context.Context` and a caller-owned
   `*sql.Conn`. It pings and identifies the server itself. It does not accept a
   product, dialect, profile, version, capability target, DSN, or transport
   connection configuration from the caller.
2. The observed server identity is authoritative. The session stores only the
   private connection and identity-derived routing state required for analysis.
   It exposes no product, dialect, profile, version, OID, manifest, resolver,
   connection, or capability getter and no JSON-visible field.
3. The session never owns or closes the connection. It may be reused while the
   caller keeps that connection open. This milestone adds no concurrent-use
   guarantee beyond the existing dialect-specific session APIs.
4. `QueryAccessRequest.Dialect` may be empty. An empty value means "use the
   observed identity." A non-empty value is only a constraint and must match
   the observed identity exactly; it never selects or overrides routing.
5. Caller-supplied `AnalysisProfile` remains forbidden for online analysis.
   Capability is derived from observed identity. Caller-supplied
   `SchemaResolver` also remains forbidden because online proof must use the
   resolver bound to the same connection.
6. The analysis method uses this fixed validation priority when multiple
   invalid conditions coexist: unavailable context/session first; non-empty
   dialect mismatch second; non-empty analysis profile third; external schema
   resolver fourth; unavailable linked capability fifth; existing mode and
   analysis validation afterward. Tests must pin the priority.
7. The constructor maps nil context, nil connection, failed liveness, and
   identity acquisition failure to
   `ErrOnlineQueryAccessSessionUnavailable`. A recognized product/version that
   is not supported by the linked capability set returns
   `ErrOnlineQueryAccessCapabilityUnsupported`. Errors remain bounded and must
   not contain credentials, endpoints, raw versions, catalog facts, OIDs,
   backend identifiers, submitted SQL, or driver text.
8. Official DeltaScope release binaries support all currently admitted online
   products: MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17. The no-PostgreSQL-
   tag Go source build retains the same unified public symbols for compile-time
   compatibility and returns
   `ErrOnlineQueryAccessCapabilityUnsupported` for an observed PostgreSQL
   target. This is not a separate official distribution contract.
9. Routing must preserve each existing proof engine exactly. MySQL/TiDB keeps
   identity-derived builtin semantic proof. PostgreSQL keeps the PG17 manifest,
   trust policy, same-connection identity and schema resolvers, exact
   `COUNT(1)` boundary, foreign-table rejection, and no-execution guarantee.
10. Existing public dialect-specific session types, constructors, analysis
    functions, sentinel errors, validation ordering, result behavior, and build
    stubs remain source-compatible and behavior-compatible. Shared private
    execution code is permitted, but the old APIs must retain their own public
    validation and error translation.
11. CLI and HTTP retain responsibility for connection selection,
    authorization, purpose checks, TLS/config resolution, opening and closing
    the online session, cancellation, and transport-specific error mapping.
    They stop switching on product only after the unified SDK entry owns that
    routing.
12. CLI exit codes and stderr, HTTP status/error codes/messages/access logs,
    authorization behavior, `connection_id` contract, timeout/cancellation
    behavior, and no-leak guarantees remain unchanged.
13. SDK, CLI, and HTTP are the only milestone surfaces. MCP continues to have
    no Query Access capability and its no-surface contract remains enforced.
14. Existing repeated SDK/CLI/HTTP behavior matrices remain in this milestone.
    Their later consolidation is tracked in GitHub issue #4 and may not remove
    transport-owned lifecycle, authorization, failure-mapping, or privacy
    evidence. Dialect-specific API deprecation/removal is separately tracked in
    issue #3.

## Explicit Non-Goals

- No new SQL shape, profile, database version, proof engine, admission result,
  requirement, reason code, or output field.
- No relationless, literal, aggregate, PostgreSQL, MySQL, or TiDB capability
  expansion.
- No direct constructor from a transport session, DSN, identity, product,
  profile, or capability target.
- No connection pool fallback and no SDK ownership of connection close.
- No session introspection getters or public identity object.
- No concurrency promise, cache, retry, SQL execution, authorization decision,
  grant/RLS/masking evaluation, or data retrieval.
- No MCP Query Access surface.
- No deprecation or removal of existing dialect-specific APIs.
- No test-matrix deletion or consolidation in this milestone.
- No removal of the source build path without the `postgresql` tag and no claim
  that it is an official PostgreSQL-disabled product edition.

## Acceptance Evidence

The ADR may change to Accepted only after all of the following exist:

- Public API and reflection tests prove the exact signatures, opacity, no
  exported fields/getters, caller connection ownership, reuse semantics, and
  fixed validation priority.
- Constructor tests cover nil context/connection, failed ping, identity
  failure, unsupported product/version, and the no-PostgreSQL-tag PostgreSQL
  fail-closed path without leaking internal facts.
- Shared contract tests compare unified results and errors with the existing
  dialect-specific paths for every supported product/profile and representative
  positive, indeterminate, rejected, failure, cancellation, no-execution, and
  no-leak cases.
- PostgreSQL recording and PG17 integration evidence proves identity, relation,
  column, and function proof remain on the same caller-owned connection;
  foreign tables and excluded `COUNT` shapes remain fail closed.
- MySQL/TiDB recording and live evidence proves identity-derived profiles,
  admitted literal shapes, relationless shapes, excluded shapes, and
  no-execution behavior remain unchanged.
- CLI and HTTP tests prove their external contracts are byte/field compatible,
  retain transport ownership, and no longer contain a product switch for
  online Query Access. Existing repeated behavior tests remain present.
- Default and PostgreSQL-tagged tests, affected race tests, build, vet, lint,
  Query Access corpus, PostgreSQL confidence, Docker-backed SDK/CLI/HTTP, module
  tidy, formatting, documentation, and diff checks pass.
- An independent read-only review reports no P0, P1, or P2 finding and confirms
  that old APIs, MCP absence, privacy, and supported SQL behavior did not
  change.
