# Query Access CLI TLS and Credential Boundary

- **Date:** 2026-07-22
- **Status:** Proposed
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
