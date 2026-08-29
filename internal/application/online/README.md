# Online Session Module

Shared online connection identity and error boundaries for the CLI, HTTP, and public Query Access session APIs.

## Files

| File | Responsibility |
|------|---------------|
| identity.go | Parses server identity, derives supported capability series, and classifies the bounded PostgreSQL PG17 version boundary |
| session.go | Opens and pins MySQL/TiDB or PostgreSQL sessions, performs liveness and identity probes, and closes owned resources |
| errors.go | Maps online operation failures to bounded HTTP codes, messages, and statuses without exposing driver or connection details |
| identity_test.go | Verifies supported, mismatched, malformed, unsupported, and bounded PostgreSQL identity behavior |
| session_test.go | Verifies identity probes, session lifecycle, DSN construction, and connection defaults |
| session_integration_test.go | Verifies real Docker-backed identity and session behavior |

## Exports

- `ProductFamily`, `VersionSeries`, `ServerIdentity`, and `CapabilityTarget`
- `ParseServerIdentity()`, `DeriveCapabilityTarget()`, and `IdentifyFromConn()`
- `SessionConfig`, `Session`, and `OpenSession()`
- `MapOnlineError()`, `IsAuthenticationFailure()`, and bounded online error sentinels
- `PostgreSQLQueryAccessVersionRequirement` and `ErrPostgreSQLQueryAccessVersionUnsupported`

## Dependencies

- Upstream: `pkg/deltascope`, `internal/interfaces/cli`, `internal/interfaces/http`
- Downstream: `database/sql`, MySQL driver, pgx/stdlib, `internal/application/auditmeta`

## Update Rule

- If members, identity contracts, error mappings, or dependencies change, update this file in the same change.
