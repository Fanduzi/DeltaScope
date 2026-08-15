# Design: Remove DB-Backed Query Access Resolvers

## Status

Proposed. This design removes unused infrastructure adapters without changing
the production Query Access contract.

## Current Shape

Each database metadata package currently has two ownership adapters:

```text
*sql.DB   -> QueryAccessResolver       -> catalog lookup
*sql.Conn -> QueryAccessConnResolver   -> catalog lookup
```

Only tests construct the `*sql.DB` adapters. Production online analysis uses
the second path because observed identity and every proof query must remain on
one caller-owned session.

## Chosen Shape

Keep one ownership boundary:

```text
caller-owned *sql.Conn
  -> QueryAccessConnResolver
  -> existing dialect catalog behavior
  -> unified Online Query Access proof
```

For PostgreSQL, `query_access_resolver_core.go` remains the private stateless
catalog implementation behind the conn adapter. If deletion leaves an unused
method or compatibility stub, remove that dead code rather than preserving it
for hypothetical pool callers.

For MySQL/TiDB, retain the existing conn resolver directly. Do not introduce a
new shared interface or refactor merely to make its file resemble PostgreSQL.

## Why Deletion Is Safe

- Exact repository search finds no production caller of either DB constructor.
- Both types live under `internal/`, so no external Go module can import them.
- Official online SDK, CLI, and HTTP paths already use a pinned `*sql.Conn`.
- Pool-backed metadata cannot satisfy the same-session proof contract.
- Preserving dead constructors would require maintaining failure behavior that
  has no supported consumer.

## Test Ownership

Build a small reconciliation table before deletion:

| Obligation | Preferred owner |
|---|---|
| table/view kind and ordered columns | conn resolver unit/integration |
| missing relation and unsupported relkind | conn/core test |
| cancellation and closed session | conn resolver lifecycle test |
| query, scan, and iteration failures | smallest conn/core recording test |
| caller ownership and same backend | conn integration test |
| trusted proof behavior | unified SDK/live integration |

The table is a review aid in the implementation plan, not a new checker or
test framework. Existing evidence wins; add only the smallest missing test.

PostgreSQL integration tests that currently pass a DB resolver into a trusted
service must pin a connection and use the conn resolver. This corrects test
wiring to match the production trust boundary without changing the service.

## Error and Privacy Boundary

Deletion does not authorize error cleanup. PostgreSQL and MySQL/TiDB currently
have intentionally different conn-backed error vocabularies. Their existing
tests remain authoritative.

No raw driver error, DSN, credential, catalog identity, backend identifier, or
user SQL may newly reach a public result or transport. No user SQL is executed.
Any production error-policy change requires a separate issue.

## Documentation

- Add this decision and its spec/design/implementation documents.
- Update both metadata READMEs and changed L3 headers.
- Add a follow-up link to the Accepted 2026-08-11 resolver-core ADR because it
  intentionally retained the PostgreSQL DB adapter pending issue #2.
- Add evidence-maintenance notes to older ADRs only where deleted test names or
  dual-resolver claims would otherwise point at current evidence incorrectly.
- Do not rewrite v0.480.0 release notes; they remain a historical snapshot.
- Do not add a domain glossary term: resolver ownership is infrastructure, not
  domain language.

## Alternatives Rejected

### Characterize and retain the DB-backed adapters

Rejected because it creates a durable contract for code with no production
caller and preserves an ownership model that cannot support trusted proof.

### Keep deprecated compatibility wrappers

Rejected because `*sql.DB` cannot be forwarded to a caller-owned pinned
session without changing semantics. A wrapper would preserve the problem.

### Delete PostgreSQL only

Rejected because the MySQL/TiDB adapter has the same test-only status. A second
issue would add tracking cost without a distinct decision.

### Normalize conn-backed errors during deletion

Rejected because that changes production behavior and expands a mechanical
dead-code removal into a privacy and compatibility milestone.
