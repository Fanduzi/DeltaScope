# Decision: Resolve CLI Metadata Ports From Explicit Dialect

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #38
Related commits:
Related tests:
- `TestAuditCommandResolvesMetadataPortByExplicitDialect`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`

## Context

The CLI always passed its MySQL-oriented `3306` default to metadata-aware
connections, even when an operator explicitly selected PostgreSQL. The CLI
reference documented PostgreSQL as using `5432`, so an omitted port could target
the wrong service before dialect handling completed.

## Decision

At the shared CLI connection-option normalization seam, preserve whether
`--port` was changed. If it was omitted and `--dialect postgresql` was explicitly
selected, resolve the port to `5432`. Explicit ports always pass through
unchanged. Explicit MySQL/TiDB and auto-detected connections keep `3306`.

The same normalized options feed audit and online query-access CLI connection
construction. No service probing is used.

## Rationale

The explicit dialect is available before opening the connection, so selecting
PostgreSQL's standard port is deterministic and requires no new abstraction or
probe. Keeping `3306` for auto-detection preserves the existing MySQL/TiDB
behavior, while changed-flag tracking prevents an implicit default from
overriding an operator's explicit value.

## Public Contract

- `--dialect postgresql` with no `--port` uses `5432` for CLI metadata/online connections.
- An explicit `--port`, including `3306`, wins for every dialect.
- Omitted ports for MySQL, TiDB, and auto-detected connections use `3306`.

## Deferred / Out Of Scope

- Probing multiple services or inferring a port from a detected PostgreSQL session.
- Changing MySQL/TiDB defaults.
- Changing database, schema, dialect-detection, or generic connection-error behavior.

## Verification Evidence

`TestAuditCommandResolvesMetadataPortByExplicitDialect` covers explicit
PostgreSQL with omitted and explicit ports, explicit MySQL/TiDB, auto-detection,
and both inline and file SQL inputs through the CLI metadata opener seam.

## Consequences

CLI help and reference documentation must describe the conditional default
rather than presenting one unconditional port. Future connection flags must
retain explicit-vs-omitted tracking at normalization boundaries.

## Links

- Commits:
- Tests: `internal/interfaces/cli/audit_metadata_test.go`
- Docs: `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`
