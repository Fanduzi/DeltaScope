# Query Access CLI TLS and Credential Boundary Specification

**Status:** Proposed
**Date:** 2026-07-22
**Target branch baseline:** `main@abb06eb` (`v0.420.0`)

## 1. Goal

Make DeltaScope CLI online metadata analysis safe and usable for both:

- `deltascope audit`
- `deltascope query-access analyze`

The CLI must support TLS to MySQL, TiDB, and PostgreSQL, and must never accept a database password as a command-line argument.

This milestone applies only when the caller intentionally supplies online connection flags. The ordinary offline CLI path remains offline and fail-closed for function-bearing Query Access SQL.

## 2. User Contract

### 2.1 Connection and TLS flags

Both commands must expose the same online connection flags:

```text
--host HOST
--port PORT
--user USER
--password-env ENV
--password-file PATH
--ask-password
--socket PATH
--database NAME
--schema NAME
--tls-mode disabled|enabled
--tls-ca-file PATH
--metadata-connect-timeout DURATION
```

`--tls-mode` defaults to `disabled`.

When `--tls-mode=enabled`:

- DeltaScope verifies both the certificate chain and hostname.
- The TLS server name is exactly `--host`; there is no hostname override flag.
- `--tls-ca-file` is optional. When absent, system trust roots are used.
- `--tls-ca-file` is parsed locally before the connection is opened.
- `--socket` is rejected because Unix sockets do not provide a TLS hostname boundary.

`--tls-ca-file` is rejected unless `--tls-mode=enabled`. Supplying TLS-only flags without a complete online connection configuration must return a bounded usage error; DeltaScope must not silently fall back to an offline analysis.

### 2.2 Credential contract

Remove `--password` and `-p` from both commands. Do not provide a compatibility alias or migration switch.

Exactly zero or one of these sources may be supplied:

- `--password-env ENV`
- `--password-file PATH`
- `--ask-password`

A password is read only after command-line validation. It must not appear in process arguments, stdout, stderr, returned JSON, logs, errors, telemetry, or test failure output.

### 2.3 Online analysis behavior

With complete online options:

- `audit` opens one metadata connection using the requested TLS mode and performs its existing metadata analysis.
- `query-access analyze` opens one online session and uses the existing trusted per-dialect Query Access path.
- User-submitted SQL is never sent to the target database. Online Query Access may run only the existing version, identity, and catalog metadata queries.

With no online options:

- existing offline behavior is unchanged;
- no database connection is created;
- function-bearing Query Access SQL remains `indeterminate` where the offline analyzer cannot prove it.

## 3. Security Requirements

1. TLS enabled mode uses `InsecureSkipVerify: false` for MySQL/TiDB and PostgreSQL `sslmode=verify-full` semantics.
2. MySQL/TiDB use a per-connection `*tls.Config`; production code must not use the driver's global `RegisterTLSConfig` registry.
3. TLS validation failures, CA parsing failures, password-source errors, connection errors, and metadata errors map to bounded CLI errors.
4. Bounded output must omit passwords, password source values and paths, CA paths, hostnames, ports, DSNs, database names, server versions, driver errors, submitted SQL, literals, identity data, and session internals.
5. The CLI must preserve the existing PostgreSQL `--database` behavior and must not inject that value into MySQL/TiDB DSNs.
6. Default `tls_mode: disabled` remains explicit and documented. Enabling TLS is opt-in, not opportunistic fallback.

## 4. Required Verification

Docker-backed CLI E2E must prove, without skips:

| Dialect | Command | Trusted CA | Untrusted CA | Hostname mismatch |
|---|---|---|---|---|
| MySQL 8.4 | audit | succeeds | bounded failure | bounded failure |
| MySQL 8.4 | query-access analyze | succeeds with manifest-proven SQL | bounded failure | bounded failure |
| PostgreSQL 17 | audit | succeeds | bounded failure | bounded failure |
| PostgreSQL 17 | query-access analyze | succeeds with `SELECT count(id) FROM app.users` | bounded failure | bounded failure |

The TLS fixture must use a private test CA and server certificates with DNS SANs. No private key or generated certificate may be tracked. The test harness must clean containers, volumes, networks, and generated temporary files on success, failure, interrupt, and timeout.

## 5. Non-Goals

This milestone does not:

- expose online Query Access through MCP;
- change HTTP registry, API-key, TLS, or credential behavior;
- execute submitted Query Access SQL or return data;
- add SQL mode attestation, grants, roles, RLS, masking, rewrite, or execution-snapshot checks;
- widen admitted function, aggregate, window, cast, literal, wildcard, view, CTE, or UDF support;
- support plaintext `--password`, a TLS hostname override, or TLS over Unix sockets;
- change the default offline SDK, CLI, or HTTP analysis path.
