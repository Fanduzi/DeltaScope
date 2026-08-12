# Design: Unified Online Query Access Analysis Entry

## Status

Proposed. This design moves product routing into the public Query Access module
without moving transport ownership or changing proof behavior.

## Existing Boundary

CLI and HTTP both perform this sequence after opening an application-owned
online session:

```text
inspect session.Identity.Product
  -> assign request dialect
  -> construct PostgreSQL or MySQL/TiDB public session wrapper
  -> call the matching dialect-specific analysis function
  -> map the result or error for the transport
```

The connection is already pinned and identified by the shared online opener,
but the public dialect-specific constructors identify it again to establish
their own trust boundary. The repeated product switch is not a transport
concern. Transport adapters should know how to obtain and authorize a
connection, not how a database product selects an internal proof engine.

## Proposed Structure

```text
CLI flags / HTTP connection_id
  -> transport authorization and configuration
  -> online.OpenSession
  -> caller-owned session.Conn
  -> NewOnlineQueryAccessSessionFromConn(ctx, conn)
       -> ping
       -> identify from the pinned connection
       -> derive private capability route
  -> AnalyzeOnlineQueryAccessWithSession(ctx, opaqueSession, request)
       -> validate generic online contract in fixed order
       -> private route selected from observed identity
          -> MySQL/TiDB existing proof core
          -> PostgreSQL existing trusted proof core
       -> QueryAccessResult
  -> transport-specific response/error mapping
  -> transport closes online.OpenSession
```

The public wrapper is opaque and stores a caller-owned `*sql.Conn` plus a
private identity-derived route. It is an SDK boundary, not a replacement for
`internal/application/online.Session`: the latter owns connection opening and
transport-neutral identity facts for application adapters, while the public
wrapper owns safe Query Access proof routing for SDK callers.

## Public Boundary

The additive public names are:

- `OnlineQueryAccessSession`;
- `NewOnlineQueryAccessSessionFromConn`;
- `AnalyzeOnlineQueryAccessWithSession`;
- five generic `ErrOnlineQueryAccess...` sentinels defined by the spec.

The session has no exported field and no public getter. It must marshal as an
empty object if passed to `encoding/json`, matching the opacity expectations of
the existing wrappers. It does not expose the observed identity so callers
cannot use the analysis session as an identity or capability discovery API.

## Identity and Dialect Semantics

Identity is observed once per unified wrapper construction from the supplied
connection. Callers cannot inject it. The private route contains only what is
needed to select an existing proof path.

An empty request dialect delegates dialect selection to observed identity. A
non-empty dialect is checked against that identity. It cannot alter the route.
This lets CLI and HTTP omit their current product-to-dialect switch while still
letting direct SDK callers assert an expected dialect.

The constructor distinguishes an unavailable session from an unsupported
capability without exposing why identity failed. Nil input, liveness failure,
or inability to obtain trustworthy identity is unavailable. A trustworthy but
unsupported product/version or an observed PostgreSQL server in a source build
without PostgreSQL capability is unsupported.

## Fixed Validation Order

`AnalyzeOnlineQueryAccessWithSession` validates in this order:

1. context and opaque session usability;
2. non-empty request dialect against observed identity;
3. empty caller analysis profile;
4. absent caller schema resolver;
5. linked capability availability;
6. existing mode and proof-path validation.

This order is a compatibility contract for the new API. Table-driven tests
must combine invalid fields so later refactors cannot accidentally expose a
different error. Context cancellation continues to be honored by the existing
operations, but the bounded public sentinel remains the first structural
validation result when context/session input itself is unavailable.

## Private Execution Cores

The implementation should extract private helpers only where needed to let old
and new public wrappers share proof assembly and execution. The helpers may
accept a pinned connection, identity-derived capability target, and sanitized
request. They must not own public validation policy.

The old public functions continue to run their current validation order and
return their current dialect-specific errors. They must not be implemented as
a blind call to the new public function followed by incomplete error mapping.
Characterization tests should pin the old behavior before extraction.

The new API translates private failures to its generic sentinels. Parsing and
analysis errors that already belong to the stable Query Access API retain their
existing typed/wrapped behavior after the online-specific validation succeeds.

## Build and Distribution Boundary

Official `deltascope`, `deltascope-server`, and `deltascope-mcp` builds and all
GoReleaser artifacts link PostgreSQL support. The design does not introduce a
second official product edition.

The repository still supports compiling `pkg/deltascope` without the
`postgresql` tag, including its default test lane and portable non-CGO source
builds. A no-tag private capability helper reports PostgreSQL as unsupported;
the PostgreSQL-tagged helper assembles the existing PG17 proof. Public unified
types and functions live in files available to both builds so SDK source shape
does not vary.

Existing dialect-specific PostgreSQL stubs remain unchanged. The unified API's
generic unsupported error does not replace
`ErrPostgreSQLSessionNotAvailable` for old callers.

## Transport Responsibility

CLI continues to own flags, TLS and credential sources, connection opening and
close, exit codes, stdout/stderr, and bounded error text. HTTP continues to own
runtime registry lookup, `connection_id` authorization/purpose checks, TLS and
credential resolution, request cancellation, status/code/message mapping, and
access logs.

After migration both transports pass the pinned connection to the unified
constructor, leave `QueryAccessRequest.Dialect` empty, and call the unified
analysis function. They do not inspect the wrapper or switch on product.

Transport adapters may add narrow translation for the new generic sentinels,
but externally visible behavior must match the current product-specific path.
In particular, constructor/capability failures remain bounded connection
failures at those surfaces; no identity, build-tag, version, endpoint, or
driver detail becomes public.

## Test Ownership

This milestone is intentionally additive. It adds a unified SDK contract and
migration equivalence tests without deleting existing dialect or transport
matrices. That gives review evidence that the new seam did not weaken the
Accepted per-surface PG17 contract.

Issue #4 owns later consolidation. Its target split is:

- unified SDK entry owns product/profile/SQL-shape/admission behavior;
- CLI owns exit codes, output streams, flags, lifecycle, and CLI no-leak;
- HTTP owns status/error JSON, authorization, registry lifecycle, access logs,
  and HTTP no-leak;
- old dialect API tests remain until issue #3 decides their lifecycle.

No test may be deleted in this milestone merely because a new equivalent test
exists.

## File Layout

Exact names may change during implementation, but the intended ownership is:

- one untagged public file for the unified opaque type, generic errors,
  constructor, validation, and routing shell;
- small tagged and untagged private PostgreSQL capability files;
- existing MySQL/TiDB and PostgreSQL session files retaining old public API
  behavior while sharing private execution helpers where safe;
- thin CLI and HTTP online Query Access adapters with no product switch;
- SDK, CLI, and HTTP tests proving equivalence and ownership boundaries;
- synchronized `pkg/deltascope` and transport module documentation.

## Privacy and Failure Semantics

The wrapper and generic errors expose no raw identity, version, DSN, host,
port, database, username, credential source, password, TLS path, OID, backend
PID, catalog query, user SQL, or driver error. The user SQL continues to be
parsed locally and must never be sent to the database.

Recording-driver tests must prove that only bounded identity and catalog probes
run. Failure tests must prove that constructor and proof failures do not leak
through SDK structs/errors, CLI streams, HTTP JSON, or HTTP access logs.

## Alternatives Rejected

### Keep routing in each transport

Rejected because product-to-proof routing is an SDK/application concern and
the duplicated switches already impose repeated migration and test cost.

### Constructor accepts identity or product

Rejected because caller-supplied identity would turn a trust fact into a hint
and could route proof through the wrong engine.

### Constructor accepts an application online session

Rejected because that internal type includes transport/application lifecycle
concerns and is not an appropriate stable public SDK dependency.

### One function that opens and closes the database connection

Rejected because it would combine transport credential/configuration policy
with static analysis and weaken caller-owned session guarantees.

### Replace old APIs immediately

Rejected because their validation and error contracts differ. Deprecation is a
separate public compatibility decision tracked in issue #3.

### Delete duplicate test matrices now

Rejected because it would combine an API migration with evidence reduction.
Issue #4 owns a later coverage-equivalence review.

### Advertise a PostgreSQL-disabled official build

Rejected because official artifacts already include PostgreSQL. The no-tag
path is source-build compatibility only.

## Documentation Impact

Implementation must update L3 headers for changed Go files and L2 READMEs for
`pkg/deltascope`, CLI, and HTTP. Public SDK documentation must describe the new
entry as additive and caller-connection-owned. Release documentation changes
belong to a later release-preparation milestone, not this implementation.
