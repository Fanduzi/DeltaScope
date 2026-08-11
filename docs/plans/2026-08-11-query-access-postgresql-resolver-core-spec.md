# Spec: PostgreSQL Query Access Resolver Core Deduplication

## Status

Proposed. This document defines an internal architecture refactor for later
implementation. It does not authorize a Query Access behavior change.

## Problem

The PostgreSQL metadata package has two `SchemaResolver` adapters:

- `QueryAccessResolver`, backed by a pool-style `*sql.DB`;
- `QueryAccessConnResolver`, backed by one caller-owned `*sql.Conn`.

The adapters represent different ownership guarantees, but they duplicate the
catalog SQL, query and row scanning, column loading, and associated error
construction. Relation-kind mapping and foreign-table rejection already have
one shared implementation, but those helpers currently live in the DB-backed
adapter file even though the conn-backed adapter also depends on them. The
remaining copies can drift, and the shared policy has no neutral owner.

Combining the adapter types would remove visible duplication at the cost of
hiding a security-relevant distinction. Trusted online PostgreSQL proof must
remain bound to a caller-owned connection and must never silently fall back to
a `*sql.DB` pool.

## Objective

Give the PostgreSQL package one private, stateless implementation of relation
metadata resolution while retaining two explicit adapters and their existing
ownership contracts.

The refactor is complete only when catalog SQL, row scanning, relation-kind
mapping, unsupported-relkind rejection, column loading, and lookup error
construction each have one production implementation. Constructors, concrete
handle fields, lifecycle checks, build-tag stubs, and ownership evidence remain
adapter-specific.

## Required Contract

1. The milestone is PostgreSQL-only. No MySQL/TiDB resolver, public SDK API,
   parser, admission rule, result field, transport behavior, or release surface
   may change.
2. `QueryAccessResolver` remains a distinct `*sql.DB`-backed type with the
   existing `NewQueryAccessResolver(*sql.DB)` constructor and
   `ResolveRelation` method signature.
3. `QueryAccessConnResolver` remains a distinct `*sql.Conn`-backed type with
   the existing `NewQueryAccessConnResolver(*sql.Conn)` constructor and
   `ResolveRelation` method signature. Its struct must not gain a `*sql.DB`
   field or any pool fallback.
4. Trusted online PostgreSQL Query Access continues to construct only the
   conn-backed resolver from the same caller-owned `*sql.Conn` used by the
   identity adapter. The DB-backed resolver remains ordinary schema metadata
   infrastructure and is not evidence for session-bound proof.
5. Shared code is private to
   `internal/infrastructure/metadata/postgresql`. It is stateless and accepts a
   minimal private query capability implemented naturally by `*sql.DB` and
   `*sql.Conn`. The two adapters retain their concrete fields rather than
   storing that capability in a common resolver object.
6. The shared implementation owns the PostgreSQL relation-kind query, column
   query, scanning, lookup error construction, and the already-shared
   `relkindToKind` and foreign-table fail-closed helpers. The adapter files must
   not retain duplicate catalog SQL or scanning behavior, and neutral policy
   helpers must not remain housed in either ownership adapter.
7. Existing observable behavior is frozen for this milestone: query order,
   accepted relation kinds, foreign-table rejection, context behavior,
   constructor behavior, error classification and text, and returned
   `RelationSchema` values must remain unchanged.
8. The non-PostgreSQL build stubs and all existing internal constructors remain
   available with their current signatures.
9. A shared parameterized behavior contract must run against both adapters.
   Adapter-specific tests must separately retain DB compatibility and these
   conn states in their current order: nil constructor input returns
   `ErrSessionNotPinned`; a nil resolver or nil internal connection returns
   `ErrSessionClosed` after the existing context check; a non-nil but already
   closed `*sql.Conn` delegates and returns the existing wrapped query-path
   error. Cancellation precedence, the exact concrete conn field shape, and
   real same-backend-session evidence must also remain covered.

## Explicit Non-Goals

- No consolidation of DB-backed and session-pinned adapter types.
- No cross-dialect resolver abstraction.
- No change to DB-backed nil handling or historical error policy. Those topics
  are tracked separately in GitHub issue #2.
- No removal of the DB-backed resolver.
- No public `catalogQueryer` interface or new package export.
- No Query Access capability expansion, authorization behavior, SQL execution,
  or output-schema change.
- No reduction of transport-level safety evidence required by existing
  Accepted Query Access decisions.

## Acceptance Evidence

The implementation may change the ADR to Accepted only after all of the
following evidence exists:

- Source inspection and automated checks show one neutral production owner for
  the relation SQL, column SQL, scanning, lookup errors, kind mapping, and
  foreign-table rejection. The evidence distinguishes newly deduplicated query
  behavior from the two policy helpers that were already shared.
- A parameterized contract proves equivalent relation results and errors for
  DB-backed and conn-backed adapters across base tables, partitioned tables,
  views, materialized views, missing relations, foreign tables, query errors,
  scan errors, iteration errors, and cancellation.
- Adapter-specific tests prove constructor compatibility; nil constructor, nil
  receiver/field, already-closed connection, and cancellation precedence; and
  that `QueryAccessConnResolver` retains exactly its unexported
  `conn *sql.Conn` field with no `*sql.DB` or interface-backed handle.
- Docker-backed PG17 tests prove relation metadata and trusted identity work on
  the same backend session and prove `relkind='f'` still fails closed before
  column or trusted `COUNT(1)` catalog proof.
- Default and PostgreSQL-tagged suites, affected race tests, build, vet, lint,
  PostgreSQL gates, formatting, module-tidy, decision-record, and three-level
  documentation checks pass.
- An independent read-only review reports no P0, P1, or P2 finding and confirms
  that MySQL/TiDB, public API, admission, output, and privacy contracts did not
  change.
