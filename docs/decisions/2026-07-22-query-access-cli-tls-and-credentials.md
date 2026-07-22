# Query Access CLI TLS and Credential Boundary

- **Date:** 2026-07-22
- **Status:** Accepted
- **Related released milestone:** v0.420.0 Online Query Access Connection Registry

## Context

v0.420.0 added a secure online HTTP boundary: clients select a preconfigured `connection_id`; the service authorizes it, resolves secrets locally, and can require verified TLS.

The CLI also has direct online paths for audit and Query Access, but its connection flags do not yet provide TLS configuration and still include a plaintext password flag. A password in process arguments can be exposed through shell history, process inspection, CI logs, or diagnostics. A direct operator CLI also needs a clear way to validate a private database CA and hostname.

## Decision

DeltaScope will add one shared secure direct-connection boundary for:

- `deltascope audit`;
- `deltascope query-access analyze`.

The boundary will:

- remove `--password` and `-p` without a compatibility switch;
- accept passwords only through `--password-env`, `--password-file`, or `--ask-password`;
- add `--tls-mode=disabled|enabled`, defaulting to `disabled`;
- add optional `--tls-ca-file` for enabled TLS;
- validate the certificate chain and the hostname specified by `--host`;
- reject TLS over Unix sockets and reject invalid TLS option combinations;
- pass an already-parsed CA pool through the existing online adapters;
- normalize errors at the CLI boundary so portable output never contains secrets or connection internals.

The direct CLI boundary remains separate from the HTTP connection registry. It does not accept an HTTP `connection_id`, provide API-key authorization, or persist credentials.

## Rationale

The CLI user has intentionally selected the database target, so requiring an HTTP-style server registry would make a local operator tool unnecessarily complex. At the same time, accepting a plaintext password or allowing certificate validation to be bypassed would make the direct path weaker than the HTTP path.

A strict enabled-TLS mode gives users an explicit secure mode without changing existing non-TLS use. Reusing existing online metadata/session adapters avoids a second SQL analysis path and preserves the rule that submitted Query Access SQL is never executed.

## Public Contract

Both `deltascope audit` and `deltascope query-access analyze` expose:

- `--tls-mode disabled|enabled` (default: `disabled`)
- `--tls-ca-file PATH` (optional, requires `--tls-mode=enabled`)
- `--password-env ENV`, `--password-file PATH`, `--ask-password` (mutually exclusive)

When `--tls-mode=enabled`:

- `--host` and `--user` are required
- `--socket` is rejected
- The certificate chain and hostname (exact `--host` value) are verified
- `--tls-ca-file` provides a custom CA pool; system trust roots are used when absent
- MySQL/TiDB use per-connection `*tls.Config` (no global `RegisterTLSConfig`)
- PostgreSQL uses `sslmode=verify-full` semantics

The `--password` and `-p` flags are removed without compatibility aliases.

## Verification Evidence

1. **Unit tests**: 3761 tests pass (`go test ./... -count=1`); 4936 PostgreSQL-tagged tests pass (`go test -tags postgresql ./... -count=1`)
2. **Race tests**: 409 tests pass with `-race` on CLI, online, and pkg packages
3. **CLI TLS normalization**: 14 focused tests prove flag validation, CA parsing, bounded errors
4. **Metadata adapter wiring**: 3 tests prove TLS flows through audit metadata adapter
5. **Query Access TLS**: 3 tests prove TLS flows through online session; DSN contains no password
6. **PostgreSQL credential hardening**: explicit `connConfig.Password` after `ParseConfig` prevents `.pgpass` fallback
7. **Docker CLI E2E**: `make test-e2e-cli-tls` — 12 cases covering MySQL 8.4 + PostgreSQL 17 x audit + query-access x trusted/untrusted/hostname-mismatch (requires Docker)
8. **Docs drift**: `make docs-example-gates` passes; no bare `--password` in public docs
9. **Static analysis**: `go vet ./...`, `go vet -tags postgresql ./...`, `gofmt` clean
10. **Oracle review**: 4 P1 findings addressed (bounded errors, pgpass, fail-closed TLS, implicit TCP defaults)
11. **Momus review**: 3 blocking issues addressed (task ordering, Makefile target, concrete QA commands)

## Consequences

### Positive

- CLI operators can connect to TLS-enabled MySQL, TiDB, and PostgreSQL instances.
- Private CAs can be supplied without committing certificates or keys.
- Passwords are not placed in command arguments.
- Audit and Query Access share connection validation and error behavior.

### Negative and Deferred

- Existing scripts using `--password` must migrate to `--password-env`, `--password-file`, or `--ask-password`.
- TLS defaults to disabled for compatibility; deployments must explicitly choose enabled TLS.
- No hostname override, insecure TLS mode, or TLS-over-socket support is provided.
- This does not add SQL mode attestation, database authorization checks, raw SQL execution, HTTP API changes, or MCP Query Access.
- Query Access promotion remains limited to already-proven dialect/version/function shapes.

## Acceptance Evidence Required

This record may become `Accepted` only when:

1. unit tests prove flag and credential-source validation;
2. Docker CLI E2E proves trusted-CA success and bounded untrusted-CA/hostname failures for both audit and Query Access on MySQL 8.4 and PostgreSQL 17;
3. E2E has zero required skips and leaves no fixture resources or generated private keys;
4. tests prove submitted Query Access SQL is not sent to a database driver;
5. Oracle, Momus, and an independent diff review have no unresolved P0/P1/P2 findings;
6. public documentation accurately describes the direct CLI boundary, secure credential sources, TLS behavior, and remaining non-goals.
