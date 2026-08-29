# Decision: Bound the PostgreSQL Online Query Access Version Boundary

Date: 2026-08-30
Status: Accepted
Related issue: [#33](https://github.com/Fanduzi/DeltaScope/issues/33)
Related commits:
- [`c862d33edf210cbf2e9a6f6b39a7b76697f8fad2`](https://github.com/Fanduzi/DeltaScope/commit/c862d33edf210cbf2e9a6f6b39a7b76697f8fad2) — implementation
- [`aa4aef8ecc956bc9a4c2ae236832c04a2615bde5`](https://github.com/Fanduzi/DeltaScope/commit/aa4aef8ecc956bc9a4c2ae236832c04a2615bde5) — implementation correction and authentication classification
- [`17fdb19cf833685bb537e4f11dbf7be8b35bcd92`](https://github.com/Fanduzi/DeltaScope/commit/17fdb19cf833685bb537e4f11dbf7be8b35bcd92) — implementation correction for shared authentication and timeout classification
Related tests:
- `TestParseServerIdentity_PostgreSQLUnsupportedVersionIsBounded`
- `TestOnlineQueryAccessSession_RecognizedButUnsupportedVersion`
- `TestQueryAccessOnlinePostgreSQL16ReportsVersionRequirement`
- `TestQueryAccessOnlineAuthenticationFailureRemainsBounded`
- `TestHandlerQueryAccessOnlinePostgreSQL16ReportsVersionRequirement`
- `TestHandlerQueryAccessOnlineKeepsAuthenticationDialAndTimeoutDistinct`
- `TestHandlerCapabilitiesReturnsSurfaceSummary`
- `TestWrapConnectionFailureKeepsAuthenticationAndDialDistinct`
- `TestQueryAccessOnlineTimeoutRemainsBounded`
Related docs:
- `docs/reference/query-access-analysis.md`
- `docs/reference/query-access-analysis_zh.md`
- `docs/reference/http-api.md`
- `docs/reference/http-api.zh-CN.md`

## Context

The online Query Access session already trusts PostgreSQL 17 only. A reachable
PostgreSQL 16 server therefore failed identity validation, but the generic
unsupported-series error was collapsed to `connection failed` by the CLI. The
HTTP adapter returned `identity_error` with a generic message, while its
capabilities payload did not advertise that code. Online database
authentication and dial failures were also both exposed as `connection_failed`
to HTTP clients. Audit connectivity and the PostgreSQL audit path were already
working and must not change.

## Decision

Keep PostgreSQL 16 and every PostgreSQL version outside the trusted PG17 series
outside the Query Access trust boundary. The shared identity parser returns a
version-specific bounded sentinel for a recognized PostgreSQL identity outside
PG17. It remains compatible with the generic unsupported-identity sentinel, but
its stable message is:

```
online PostgreSQL Query Access requires PostgreSQL 17
```

The unified SDK session exposes the corresponding bounded version sentinel.
The online CLI maps it to exit `3` and the message above. HTTP maps it to the
documented `identity_error` code with status `502`, and `/v1/capabilities`
advertises that code. The shared opener classifies database authentication as
`authentication_failed` while dial, timeout, and TLS mappings retain separate
bounded categories; HTTP advertises the authentication code as well.

## Rationale

The server has already accepted the connection, so reporting the trusted
version boundary is more useful than presenting a transport failure. Reusing
the existing HTTP `identity_error`/502 contract avoids inventing a second
identity code while making the previously undocumented behavior discoverable.
Keeping the classification in the shared identity path gives the CLI, HTTP,
and SDK the same source of truth without touching transport connection or
database/schema semantics.

## Public Contract

- PostgreSQL 17 trusted Query Access, PostgreSQL audit, MySQL/TiDB Query Access,
  and the existing #36 CLI flag ownership remain unchanged.
- A reachable PostgreSQL identity outside PG17 returns the bounded requirement
  message above; the CLI exits `3` and emits no Query Access result.
- HTTP returns `502` with `{"error":{"code":"identity_error","message":"online PostgreSQL Query Access requires PostgreSQL 17"}}` for the version boundary and the same `error` envelope with code `authentication_failed` for database authentication; both codes are advertised in `structured_errors`.
- Authentication, dial, timeout, and TLS failures remain distinguishable at
  their existing surfaces.
- No response, error, or access log exposes credentials, DSNs, submitted SQL,
  observed raw versions, catalog/object names, OIDs, or driver text.

## Deferred / Out Of Scope

- Adding PostgreSQL 16 or another version to the trusted Query Access manifest.
- Expanding PostgreSQL Query Access SQL semantics or changing database/schema
  selection.
- Adding MCP Query Access, executing user SQL, returning rows, or evaluating
  authorization, grants, RLS, or masking.
- Reworking the existing #41 TLS error taxonomy or adding a new HTTP identity
  code.

## Verification Evidence

- `make test` passed on implementation commit
  `c862d33edf210cbf2e9a6f6b39a7b76697f8fad2` and correction commit
  `aa4aef8ecc956bc9a4c2ae236832c04a2615bde5`.
- `go test -tags=postgresql ./...` passed on correction commit
  `aa4aef8ecc956bc9a4c2ae236832c04a2615bde5`.
- `go test -race` on affected packages, `go vet ./...`, and `make lint` passed
  on correction commits `aa4aef8ecc956bc9a4c2ae236832c04a2615bde5` and
  `17fdb19cf833685bb537e4f11dbf7be8b35bcd92`.
- `make test` and `go test -tags=postgresql ./...` passed on final correction
  commit `17fdb19cf833685bb537e4f11dbf7be8b35bcd92`.
- Focused default and tagged tests cover PG16 identity classification, PG17
  routing, CLI/HTTP parity, capabilities discovery, auth/no-leak boundaries,
  and unchanged transport error categories.
- The implementation changes only the shared identity classification, bounded
  surface mapping, capability discovery, tests, and synchronized references.

## Consequences

Operators and clients now receive an actionable, stable explanation when a
reachable PostgreSQL 16 server is used for online Query Access. Future
PostgreSQL trust expansion requires a separate manifest and contract decision;
changing only transport wording is insufficient.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/33
- Implementation: [c862d33edf210cbf2e9a6f6b39a7b76697f8fad2](https://github.com/Fanduzi/DeltaScope/commit/c862d33edf210cbf2e9a6f6b39a7b76697f8fad2)
- Implementation correction: [aa4aef8ecc956bc9a4c2ae236832c04a2615bde5](https://github.com/Fanduzi/DeltaScope/commit/aa4aef8ecc956bc9a4c2ae236832c04a2615bde5)
- Timeout/classifier correction: [17fdb19cf833685bb537e4f11dbf7be8b35bcd92](https://github.com/Fanduzi/DeltaScope/commit/17fdb19cf833685bb537e4f11dbf7be8b35bcd92)
- Online identity: `internal/application/online/identity.go`
- Online error mapping: `internal/application/online/errors.go`
- CLI/HTTP boundaries: `internal/interfaces/cli/audit.go`, `internal/interfaces/http/query_access.go`
