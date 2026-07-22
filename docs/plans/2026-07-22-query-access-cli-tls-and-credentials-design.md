# Query Access CLI TLS and Credential Boundary Design

**Status:** Proposed
**Date:** 2026-07-22
**Companion specification:** `2026-07-22-query-access-cli-tls-and-credentials-spec.md`

## Context

The online HTTP path already selects registered connections and carries TLS configuration through a bounded error boundary. The CLI has an intentionally direct connection model, but it currently accepts a plaintext `--password` flag and has no TLS options. That leaves the two online CLI commands unable to use the same certificate-validation posture as the HTTP service.

The correct boundary is not to turn the CLI into an HTTP registry client. The CLI caller deliberately chooses a target. The CLI must make that choice explicit, validate its credentials and TLS settings locally, then pass an opaque, normalized connection configuration into the existing online adapters.

## Decision

Create one shared CLI online-connection normalization path used by both `audit` and `query-access analyze`.

The normalized value includes:

- dialect, host, port, socket, user, schema, and PostgreSQL database;
- password resolved from exactly one non-argument source;
- TLS mode;
- an optional already-parsed `*x509.CertPool`;
- connect timeout.

The value is ephemeral. It must not be marshalled, logged, placed in Cobra errors, or retained outside command execution.

## Flag Semantics

### Passwords

Remove `--password` and `-p`.

`--password-env`, `--password-file`, and `--ask-password` remain mutually exclusive. Their values are read only after semantic flag validation. The command must not name a source path or environment variable in a result/error that may be externally captured.

### TLS

`--tls-mode` accepts only `disabled` and `enabled`.

`disabled` is the default, maintaining historical direct CLI behavior. `enabled` requires a TCP host connection. It rejects `--socket`; it uses the exact host as the TLS server name; it obtains roots from `--tls-ca-file` or the operating system trust store.

The CLI intentionally has no `--tls-server-name`, `--insecure-skip-verify`, or implicit downgrade behavior. Such flags weaken the one property this milestone adds: the server the user requested is the server whose certificate is validated.

## Adapter Mapping

The shared normalized configuration maps as follows:

```text
CLI flags
  -> validate and resolve local secrets/CA
  -> normalized CLI online connection
  -> online.SessionConfig for Query Access
  -> audit metadata ConnectionConfig for audit
  -> dialect adapter
```

For MySQL/TiDB, the adapter builds a fresh `tls.Config` and supplies it directly through the driver's connector. No process-global TLS registration is allowed.

For PostgreSQL, the adapter receives the CA pool and configures certificate and hostname verification equivalent to `sslmode=verify-full`.

Only PostgreSQL receives the CLI `--database` field. MySQL/TiDB retain their existing schema/DSN behavior.

## Error Boundary

Flag validation failures are usage errors. CA parsing, credential loading, TLS handshakes, database connection, version discovery, and metadata failures are normalized before crossing CLI command boundaries.

The safe CLI output describes the category only, for example:

- `invalid connection options`
- `invalid TLS configuration`
- `connection failed`
- `metadata analysis failed`

It must not reveal the original driver error. Debugging belongs in a caller-controlled local environment, not in a portable CLI output contract.

## Test Design

Use the existing TLS Docker fixture or extend it so the CLI is invoked from a container on the same Docker network. The databases should not need host ports for this test.

Each dialect has:

1. trusted CA and hostname success;
2. an untrusted CA negative control;
3. a hostname SAN mismatch negative control.

For Query Access success, use only SQL already proven by that dialect's online analyzer. PostgreSQL uses `SELECT count(id) FROM app.users`; MySQL uses the corresponding manifest-proven aggregate. Audit success may use existing non-executing metadata-aware inputs.

A recording-driver unit test verifies submitted SQL is never sent to a database connection. CLI tests separately verify removal of `--password`, mutual exclusion of password sources, TLS/socket incompatibility, CA-file validation, and bounded errors.

## Consequences

Users can operate the CLI against TLS-enabled databases without placing passwords in shell history or process listings. Operators who need a custom CA can provide it locally. Existing users without TLS flags retain the prior direct connection behavior.

The CLI remains a direct operator tool. It is not a multi-tenant credential store, does not inherit HTTP connection IDs, and does not create a new authorization layer. The server-side HTTP registry remains the deployment option for shared platforms.
