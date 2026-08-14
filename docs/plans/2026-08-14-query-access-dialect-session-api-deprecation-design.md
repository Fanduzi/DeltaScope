# Design: Deprecate Dialect-Specific Query Access Session APIs

## Status

Proposed. This design narrows the recommended SDK surface without changing the
compiled compatibility surface or runtime behavior.

## Existing Boundary

The Accepted unified-entry decision established one online SDK path across
MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17:

```text
caller-owned *sql.Conn
  -> NewOnlineQueryAccessSessionFromConn
  -> AnalyzeOnlineQueryAccessWithSession
  -> existing identity-selected proof engine
```

CLI and HTTP already use that path. The dialect-specific constructors,
sessions, and analyzers now serve compatibility callers and retained comparison
tests; they are no longer needed as a second recommended design.

## Deprecation Mechanism

Use Go's native documentation convention and nothing else:

```go
// Deprecated: Use NewOnlineQueryAccessSessionFromConn.
```

The marker belongs on both old session types and all four entry functions. The
PostgreSQL constructor and analyzer exist in tagged and untagged files, so both
declarations receive matching notices. This keeps `go doc`, editors, and Go
language tooling consistent in either source build.

No annotation package, build-time checker, runtime warning, environment
variable, feature flag, or logging hook is needed. Deprecation is guidance, not
a new execution path.

## Compatibility Boundary

The old declarations remain where they are and keep their current bodies. They
must not be rewritten as thin calls to the unified API because their validation
priority and error vocabulary are distinct public compatibility contracts.

The following stay unchanged:

- exported names and signatures;
- tagged and untagged PostgreSQL availability;
- dialect-specific sentinel identities and error text;
- validation order and wrapping;
- caller ownership of `*sql.Conn`;
- result equivalence and proof boundaries;
- no user SQL execution and no private-data leakage.

Error sentinels are not deprecated. A consumer that has not yet migrated must
still match the old errors correctly. Migration documentation explains that the
unified API returns its own `ErrOnlineQueryAccess...` sentinels and that callers
must update `errors.Is` branches rather than expect a one-to-one error alias.

## Documentation Shape

Current documentation has one canonical flow:

```go
session, err := deltascope.NewOnlineQueryAccessSessionFromConn(ctx, conn)
if err != nil {
    // Match bounded ErrOnlineQueryAccess... sentinels with errors.Is.
}

result, err := deltascope.AnalyzeOnlineQueryAccessWithSession(
    ctx,
    session,
    deltascope.QueryAccessRequest{
        SQL:           sqlText,
        Mode:          deltascope.QueryAccessModeStrict,
        DefaultSchema: schema,
    },
)
```

The request dialect is omitted in the canonical example because observed
identity owns routing. Documentation may mention that a matching non-empty
dialect is an assertion, not a routing selector.

One short migration section maps:

| Deprecated | Replacement |
|---|---|
| `PostgreSQLQueryAccessSession` | `OnlineQueryAccessSession` |
| `MySQLTiDBQueryAccessSession` | `OnlineQueryAccessSession` |
| `NewPostgreSQLQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `NewMySQLTiDBQueryAccessSessionFromConn` | `NewOnlineQueryAccessSessionFromConn` |
| `AnalyzePostgreSQLQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |
| `AnalyzeMySQLTiDBQueryAccessWithSession` | `AnalyzeOnlineQueryAccessWithSession` |

The migration section does not preserve two full tutorials. Historical release
notes and ADRs retain the old names because they describe what existed at that
time.

## Test Ownership

No new behavior matrix is required for documentation-only deprecation. Existing
old-API tests remain the compatibility evidence; existing unified-versus-old
tests prove the recommended replacement is behaviorally equivalent within its
documented generic error boundary.

A focused source or `go doc` assertion may pin the six `Deprecated:` markers if
the repository's test style supports it, but it must stay small and must not
parse every documentation page. Documentation accuracy is verified directly by
the review and documentation gates.

Issue #4 owns future test consolidation. It may centralize product and SQL-shape
matrices under the unified API while leaving a small old-API contract suite.
This milestone performs no deletion.

## Removal Boundary

Deprecation starts no countdown. A future removal is a separate breaking-change
decision, not an implementation step hidden in this plan. It requires current
usage research, explicit migration-cost analysis, a new issue, and a Proposed
ADR. Pre-1.0 versioning alone is not sufficient justification.

## File Impact

Expected production changes are comment-only in the files that declare the two
types and four functions, including the PostgreSQL tagged/untagged pair.
Current public SDK/reference/recipe documentation is updated in English and
Chinese. L3 headers and L2 package documentation are synchronized only where
the repository's three-level documentation rules require it.

No transport, application, domain, infrastructure, workflow, release, or
dependency file should change.

## Alternatives Rejected

### Retain both APIs as equally recommended

Rejected because transports and the public SDK now have one reviewed routing
boundary. Two recommended paths add navigation and maintenance cost without a
distinct capability.

### Remove the old APIs now

Rejected because public GitHub search cannot observe private consumers and the
repository has no explicit breaking-release policy that justifies an immediate
source break.

### Set a removal version

Rejected because a date without usage and migration evidence is an arbitrary
promise. A future removal decision must stand on its own evidence.

### Deprecate old error sentinels

Rejected because compatibility callers still need those sentinels until they
migrate, and the unified error vocabulary is intentionally not identical.

### Emit runtime warnings

Rejected because library analysis should not write unsolicited output or add
observable behavior merely to advertise documentation guidance.

### Delete duplicate tests with the deprecation

Rejected because issue #4 separately owns evidence consolidation. Mixing it
with API lifecycle work would make a compatibility review harder.
