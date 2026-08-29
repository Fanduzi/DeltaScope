# Decision: Bound TLS Metadata Connection Error Categories

Date: 2026-08-30
Status: Accepted
Related issue: [#41](https://github.com/Fanduzi/DeltaScope/issues/41)
Related commits:
- `fix(cli): classify TLS metadata connection failures (#41)`
Related tests:
- `TestAuditTLSFailureCategories`
- `TestAuditTLSFailureExitsRuntime`
- `TestAuditCommandLoadsTLSCAFile`
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`
- `README.md`
- `README_ZH.md`

## Context

TLS failures from the MySQL and PostgreSQL metadata openers were reduced to either
`TLS certificate verification failed` or `TLS handshake failed`. That hid the
distinction between a hostname mismatch, an untrusted CA, and a server that did not
offer TLS. The existing CLI contract also requires one bounded stderr line and exit
3 for runtime connection failures.

## Decision

Classify metadata-audit connection errors at the CLI boundary using Go's typed
`x509.HostnameError` and `x509.UnknownAuthorityError` when available, plus the
stable driver signals for a server that refuses or does not support TLS. Preserve
the existing generic verification and handshake fallbacks.

The bounded messages are:

- `TLS hostname mismatch`
- `TLS unknown certificate authority`
- `TLS server did not offer TLS`
- `TLS certificate verification failed`
- `TLS handshake failed`

Every category remains a runtime error with exit code 3. Certificate verification,
including hostname verification, remains enabled. The mapper never renders the
target host or address, certificate identity, credentials, DSN, CA-file path, or raw
driver error.

## Rationale

Typed x509 errors provide a stable cause without requiring their sensitive fields to
be printed. The two database drivers expose different no-TLS signals, so the shared
CLI mapper recognizes only their bounded wording. Keeping the change at the existing
CLI audit boundary preserves the separate query-access, HTTP, MCP, and SDK contracts.

## Public Contract

`deltascope audit` reports the five TLS categories above on stderr and exits 3. All
other non-TLS connection, authentication, timeout, and password-source mappings are
unchanged.

## Deferred / Out Of Scope

- Printing hostnames, addresses, certificate CN/SAN values, DSNs, paths, credentials,
  or raw driver diagnostics.
- Disabling hostname verification or adding an insecure TLS mode.
- Adding `--tls` or `--ssl-ca` aliases.
- Changing query-access, HTTP, MCP, or SDK error contracts.
- Repairing database-container certificate initialization.

## Verification Evidence

Focused CLI TLS, exit-code, and no-leak tests cover typed hostname mismatch and
unknown-CA errors, MySQL and PostgreSQL no-TLS signals, generic verification and
handshake fallbacks, and exit 3. The CLI reference and bilingual quick-start docs
explain why stock MySQL 8.4 auto-generated certificates without hostname-valid SANs
cannot verify for `localhost` or an IP merely by supplying `ca.pem`.

The implementation passed `go vet ./...`, `make test`, `make pg-unit-test-gates`,
the default and PostgreSQL-tagged CLI/HTTP/MCP no-leak test selections, and
`make docs-example-gates`. `make test-e2e-cli-tls` passed all 12 scoped Docker
TLS cases and verified cleanup; the staged three-level documentation check also
passed.

## Consequences

Future metadata TLS diagnostics must add a bounded category and regression test rather
than interpolate driver text. If a driver changes its no-TLS signal, update the
classifier test and this contract without exposing the new raw error.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/41
- Mapping: `internal/interfaces/cli/audit.go`
- Tests: `internal/interfaces/cli/cli_metadata_connection_exit_test.go`
