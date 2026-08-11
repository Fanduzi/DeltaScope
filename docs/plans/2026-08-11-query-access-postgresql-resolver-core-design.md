# Design: PostgreSQL Query Access Resolver Core Deduplication

## Status

Proposed. This design deepens the PostgreSQL metadata module without changing
its external or Query Access behavior.

## Existing Boundary

`QueryAccessResolver` stores a `*sql.DB`. It can resolve relation metadata, but
the pool does not guarantee that consecutive operations use one backend
session.

`QueryAccessConnResolver` stores a caller-owned `*sql.Conn`. The trusted online
PostgreSQL SDK creates it from the same connection used by the pinned identity
adapter. This concrete ownership is part of the proof boundary, not an
incidental implementation detail.

Both adapters currently repeat the PostgreSQL catalog SQL, query and row
scanning, column loading, and associated error construction. The kind mapping
and foreign-table guard are already shared, but are defined in the DB-backed
adapter file rather than a neutral core. Their ownership semantics are
different; their relation-resolution semantics are not intended to be
different.

## Proposed Structure

```text
QueryAccessResolver{db *sql.DB}
  -> adapter-specific lifecycle behavior
  -> resolvePostgreSQLRelation(ctx, db, schema, name)

QueryAccessConnResolver{conn *sql.Conn}
  -> adapter-specific lifecycle behavior
  -> resolvePostgreSQLRelation(ctx, conn, schema, name)

resolvePostgreSQLRelation
  -> private catalogQueryer capability
  -> one relkind query and scan
  -> one unsupported-relkind guard
  -> one kind mapping
  -> one column query and scan
  -> RelationSchema
```

The private capability contains only the `QueryRowContext` and `QueryContext`
methods required by the algorithm. Both `*sql.DB` and `*sql.Conn` already
satisfy it. The shared code is a set of stateless private functions, not a
third resolver object.

The adapter structs continue to hold concrete handles. This makes accidental
pool use visible in code review and preserves the structural test that the
conn-backed resolver has no `*sql.DB` field.

## Responsibility Split

### DB-backed adapter

- Keeps the existing type, constructor, method signature, and concrete
  `*sql.DB` field.
- Keeps its existing constructor and lifecycle behavior, including historical
  edge behavior deferred to issue #2.
- Delegates only the catalog-resolution algorithm to the shared core.
- Remains ineligible as evidence for session-bound identity proof.

### Conn-backed adapter

- Keeps the existing type, constructor, method signature, and concrete
  `*sql.Conn` field.
- Preserves the current state distinctions and ordering: nil constructor input
  returns `ErrSessionNotPinned`; `ResolveRelation` checks context first; a nil
  resolver or nil internal connection then returns `ErrSessionClosed`; a
  non-nil but already-closed `*sql.Conn` delegates and returns the existing
  wrapped query-path error.
- Delegates only the catalog-resolution algorithm to the shared core.
- Remains the only schema adapter assembled into the caller-owned trusted
  PostgreSQL session path.

### Shared core

- Owns the exact catalog SQL and query order.
- Scans relation kind and columns into application-layer schema facts.
- Rejects foreign tables before the column query.
- Maps only `r` and `p` to table, and `v` and `m` to view.
- Produces the existing lookup and scan errors without introducing a new error
  policy.
- Has no exported symbol, no stored database handle, no cache, and no trust or
  admission decision.

The `dialect` parameter remains on the adapter method because it is part of the
existing `SchemaResolver` interface. It is not added to the private core because
the implementation is PostgreSQL-specific and does not branch on dialect.

## File Layout

The intended production layout is:

- `query_access_resolver.go`: thin DB-backed adapter;
- `query_access_conn_resolver.go`: thin conn-backed adapter;
- `query_access_resolver_core.go`: private query capability and shared catalog
  algorithm, under the PostgreSQL build tag;
- existing non-PostgreSQL stub files: unchanged public internal shapes.

The exact helper names may change during implementation, but the ownership and
visibility boundaries above may not.

## Error and Privacy Semantics

This milestone does not redesign errors. Shared functions must preserve the
current PostgreSQL error messages and wrapping behavior for missing relations,
relation-kind lookup failures, column lookup failures, scan failures, and row
iteration failures. Adapter-specific lifecycle errors remain in their adapter.

No shared error may contain new catalog facts, OIDs, connection values,
credentials, submitted SQL, or driver details beyond what the current
PostgreSQL resolver already returns internally. Existing public application
and transport error mapping remains unchanged.

## Test Design

One parameterized behavior contract owns relation-resolution semantics. It
constructs both adapters over equivalent test query handles and runs the same
cases against each. This prevents the tests themselves from becoming two
manually synchronized copies.

Thin adapter tests retain evidence that cannot be generalized:

- DB constructor and field compatibility;
- conn nil-constructor, nil receiver/field, already-closed connection, and
  cancellation-precedence behavior;
- conn adapter's exact unexported `conn *sql.Conn` field shape and structural
  absence of `*sql.DB` or an interface-backed handle;
- caller ownership and no connection close by the resolver;
- real PG17 same-backend-PID behavior.

Docker-backed PG17 tests remain mandatory because a fake query capability
cannot prove that the conn-backed adapter and identity resolver execute on the
same backend session. The existing foreign-table fixture must also prove that
rejection occurs before column or trusted `COUNT(1)` probes.

## Alternatives Rejected

### One resolver with an interface field

Rejected because it hides whether the runtime value is a pool or one pinned
connection. Constructor discipline alone is weaker and harder to review than
two concrete ownership types.

### Merge the two adapter types

Rejected because it erases the distinction that protects session-bound proof
from pool fallback.

### Cross-dialect catalog core

Rejected because PostgreSQL and MySQL/TiDB differ in placeholders, catalog
shape, relation kinds, fail-closed policy, and current error behavior. A common
interface would obscure those differences for little reuse.

### Remove the DB-backed resolver

Rejected for this milestone because it is an existing internal adapter used by
tests and integration paths. Removal is a separate compatibility decision.

### Fix historical edge behavior during extraction

Rejected because constructor and error-policy changes would make a structural
refactor behaviorally ambiguous. GitHub issue #2 owns that follow-up.

## Documentation Impact

Implementation must update the L3 headers of changed Go files and the
PostgreSQL metadata module README. No product, transport, release, or public SDK
documentation should change because the supported behavior is unchanged.
